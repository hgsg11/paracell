package usecase

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hgsg11/paracell/internal/adapter/state"
	"github.com/hgsg11/paracell/internal/domain"
)

func TestRetryCellは同じCellの外部Resource処理を一実行だけに制限する(t *testing.T) {
	store := newRetryTestStore(t, failedRetryTestCell("cell-1", "123"))
	ports := newBlockingRetryPorts(2)
	first := newConcurrentRetryUseCase(store, ports, "attempt-1")
	second := newConcurrentRetryUseCase(store, ports, "attempt-2")
	firstResult := make(chan error, 1)
	go func() {
		_, err := first.Execute(context.Background(), RetryCellInput{Cell: "123"})
		firstResult <- err
	}()

	waitForEnteredCell(t, ports.entered, "123")
	_, err := second.Execute(context.Background(), RetryCellInput{Cell: "123"})
	if err == nil || !strings.Contains(err.Error(), "retry already in progress") {
		t.Fatalf("competing retry error = %v", err)
	}
	select {
	case cell := <-ports.entered:
		t.Fatalf("competing retry started external work for %q", cell)
	case <-time.After(50 * time.Millisecond):
	}
	ports.release <- struct{}{}
	if err := <-firstResult; err != nil {
		t.Fatalf("winning retry failed: %v", err)
	}
	if got := ports.createCount("123"); got != 1 {
		t.Fatalf("external create count = %d, want 1", got)
	}
}

func TestRetryCellは異なるCellの外部Resource処理を並行実行する(t *testing.T) {
	store := newRetryTestStore(t,
		failedRetryTestCell("cell-1", "123"),
		failedRetryTestCell("cell-2", "456"),
	)
	ports := newBlockingRetryPorts(2)
	results := make(chan error, 2)
	for _, run := range []struct {
		cell    string
		attempt string
	}{{"123", "attempt-1"}, {"456", "attempt-2"}} {
		go func(cell string, attempt string) {
			_, err := newConcurrentRetryUseCase(store, ports, attempt).Execute(context.Background(), RetryCellInput{Cell: cell})
			results <- err
		}(run.cell, run.attempt)
	}

	entered := map[string]bool{}
	for len(entered) < 2 {
		select {
		case cell := <-ports.entered:
			entered[cell] = true
		case <-time.After(2 * time.Second):
			t.Fatalf("different cells did not reach external work concurrently: %#v", entered)
		}
	}
	ports.release <- struct{}{}
	ports.release <- struct{}{}
	for range 2 {
		if err := <-results; err != nil {
			t.Fatalf("retry failed: %v", err)
		}
	}
}

func TestRetryCellはStaleLeaseを引き継ぎ古いAttemptの全更新を拒否する(t *testing.T) {
	started := time.Date(2026, 8, 10, 1, 2, 3, 0, time.FixedZone("JST", 9*60*60))
	old := failedRetryTestCell("cell-1", "123")
	old.BeginRetry("old-attempt", started)
	store := newRetryTestStore(t, old)
	ports := newBlockingRetryPorts(1)
	now := started.Add(defaultRetryLeaseTimeout + time.Nanosecond)
	uc := newConcurrentRetryUseCase(store, ports, "new-attempt")
	uc.Now = func() time.Time { return now }
	result := make(chan error, 1)
	go func() {
		_, err := uc.Execute(context.Background(), RetryCellInput{Cell: "123"})
		result <- err
	}()
	waitForEnteredCell(t, ports.entered, "123")

	current := loadSingleRetryTestCell(t, store)
	if current.Creation.AttemptID != "new-attempt" || !current.Creation.LeaseStartedAt.Equal(now.UTC()) {
		t.Fatalf("new lease = %#v", current.Creation)
	}
	checkpoint := cloneCell(old)
	checkpoint.CompleteCreationStage(domain.CreationStageSession)
	failed := cloneCell(old)
	failed.FailCreation(domain.CreationStageSession, errors.New("old failure"))
	succeeded := cloneCell(old)
	succeeded.FinishCreation()
	for name, update := range map[string]domain.Cell{
		"checkpoint": checkpoint,
		"failure":    failed,
		"success":    succeeded,
	} {
		if err := replaceRetryCell(context.Background(), store, update, "old-attempt"); err == nil || !strings.Contains(err.Error(), "ownership lost") {
			t.Fatalf("old %s update error = %v", name, err)
		}
	}
	oldUC := newConcurrentRetryUseCase(store, ports, "old-attempt")
	if err := oldUC.heartbeat(context.Background(), old, "old-attempt"); err == nil || !strings.Contains(err.Error(), "ownership lost") {
		t.Fatalf("old heartbeat error = %v", err)
	}
	current = loadSingleRetryTestCell(t, store)
	if current.Creation.AttemptID != "new-attempt" || current.Creation.LastError == "old failure" {
		t.Fatalf("old attempt overwrote state: %#v", current.Creation)
	}

	ports.release <- struct{}{}
	if err := <-result; err != nil {
		t.Fatalf("takeover retry failed: %v", err)
	}
}

func TestRetryCellは有効期限内のLeaseを拒否する(t *testing.T) {
	started := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	cell := failedRetryTestCell("cell-1", "123")
	cell.BeginRetry("attempt-1", started)
	store := newRetryTestStore(t, cell)
	ports := newBlockingRetryPorts(1)
	uc := newConcurrentRetryUseCase(store, ports, "attempt-2")
	uc.Now = func() time.Time { return started.Add(defaultRetryLeaseTimeout) }

	_, err := uc.Execute(context.Background(), RetryCellInput{Cell: "123"})
	if err == nil || !strings.Contains(err.Error(), "retry already in progress") {
		t.Fatalf("retry at lease boundary error = %v", err)
	}
	if got := ports.createCount("123"); got != 0 {
		t.Fatalf("external create count = %d, want 0", got)
	}
}

func TestRetryCellはHeartbeatを更新する(t *testing.T) {
	started := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	var clockMu sync.RWMutex
	now := started
	store := newRetryTestStore(t, failedRetryTestCell("cell-1", "123"))
	ports := newBlockingRetryPorts(1)
	uc := newConcurrentRetryUseCase(store, ports, "attempt-1")
	uc.HeartbeatInterval = time.Millisecond
	uc.Now = func() time.Time {
		clockMu.RLock()
		defer clockMu.RUnlock()
		return now
	}
	result := make(chan error, 1)
	go func() {
		_, err := uc.Execute(context.Background(), RetryCellInput{Cell: "123"})
		result <- err
	}()
	waitForEnteredCell(t, ports.entered, "123")
	heartbeatAt := started.Add(30 * time.Second)
	clockMu.Lock()
	now = heartbeatAt
	clockMu.Unlock()
	waitForRetryState(t, store, func(cell domain.Cell) bool {
		return cell.Creation.LeaseHeartbeatAt != nil && cell.Creation.LeaseHeartbeatAt.Equal(heartbeatAt)
	})
	ports.release <- struct{}{}
	if err := <-result; err != nil {
		t.Fatalf("retry failed: %v", err)
	}
	if defaultRetryHeartbeatInterval != 10*time.Second || defaultRetryLeaseTimeout != 2*time.Minute {
		t.Fatalf("retry timing defaults = %s / %s", defaultRetryHeartbeatInterval, defaultRetryLeaseTimeout)
	}
}

func TestRetryCellはContextCancellationでFailedへ戻る(t *testing.T) {
	store := newRetryTestStore(t, failedRetryTestCell("cell-1", "123"))
	ports := newBlockingRetryPorts(1)
	uc := newConcurrentRetryUseCase(store, ports, "attempt-1")
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := uc.Execute(ctx, RetryCellInput{Cell: "123"})
		result <- err
	}()
	waitForEnteredCell(t, ports.entered, "123")
	cancel()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error = %v", err)
	}
	cell := loadSingleRetryTestCell(t, store)
	if cell.CreationStatus() != domain.CreationFailed || cell.Creation.AttemptID != "" || cell.Creation.LastError != context.Canceled.Error() {
		t.Fatalf("creation after cancellation = %#v", cell.Creation)
	}
}

func failedRetryTestCell(id string, issue string) domain.Cell {
	cell := domain.Cell{ID: id, Issue: issue, Name: issue, Template: "webapp"}
	cell.BeginCreation("fix issue")
	cell.CompleteCreationStage(domain.CreationStageSource)
	cell.CompleteCreationStage(domain.CreationStageFiles)
	cell.CompleteCreationStage(domain.CreationStageContainers)
	cell.FailCreation(domain.CreationStageSession, errors.New("tmux failed"))
	return cell
}

func newRetryTestStore(t *testing.T, cells ...domain.Cell) state.SQLiteCellStateAdapter {
	t.Helper()
	store := state.SQLiteCellStateAdapter{Path: filepath.Join(t.TempDir(), ".paracell", "state.db")}
	if err := store.SaveCells(context.Background(), cells); err != nil {
		t.Fatal(err)
	}
	return store
}

func loadSingleRetryTestCell(t *testing.T, store state.SQLiteCellStateAdapter) domain.Cell {
	t.Helper()
	cells, err := store.LoadCells(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(cells) != 1 {
		t.Fatalf("cells = %#v", cells)
	}
	return cells[0]
}

func waitForRetryState(t *testing.T, store state.SQLiteCellStateAdapter, ready func(domain.Cell) bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if ready(loadSingleRetryTestCell(t, store)) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out waiting for retry state")
}

func waitForEnteredCell(t *testing.T, entered <-chan string, want string) {
	t.Helper()
	select {
	case got := <-entered:
		if got != want {
			t.Fatalf("entered cell = %q, want %q", got, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("cell %q did not enter external work", want)
	}
}

type blockingRetryPorts struct {
	entered chan string
	release chan struct{}
	mu      sync.Mutex
	creates map[string]int
}

func newBlockingRetryPorts(capacity int) *blockingRetryPorts {
	return &blockingRetryPorts{
		entered: make(chan string, capacity),
		release: make(chan struct{}, capacity),
		creates: map[string]int{},
	}
}

func (p *blockingRetryPorts) Load(context.Context, *domain.TemplateVars) (domain.Config, error) {
	return domain.Config{
		Project: domain.ProjectConfig{Name: "project"},
		Templates: map[string]domain.Template{
			"webapp": {Name: "webapp", Session: domain.SessionTemplate{Windows: []domain.SessionWindowTemplate{{Name: "shell"}}}},
		},
	}, nil
}

func (p *blockingRetryPorts) NewCell(id string, issue string, template domain.Template, project string) (domain.Cell, error) {
	return domain.Cell{ID: id, Issue: issue, Name: issue, Template: template.Name, Session: domain.Session{Name: project + "-" + issue}}, nil
}

func (p *blockingRetryPorts) Source(domain.ProviderConfig) (SourcePort, error)       { return p, nil }
func (p *blockingRetryPorts) Container(domain.ProviderConfig) (ContainerPort, error) { return p, nil }
func (p *blockingRetryPorts) Session(domain.ProviderConfig) (SessionPort, error)     { return p, nil }
func (p *blockingRetryPorts) ResumeSource(context.Context, domain.Cell) error        { return nil }
func (p *blockingRetryPorts) CreateSource(context.Context, domain.Cell) (SourceCreation, error) {
	return SourceCreation{}, nil
}
func (p *blockingRetryPorts) CleanSource(context.Context, domain.Cell) error { return nil }
func (p *blockingRetryPorts) CopyFiles(context.Context, domain.Cell, domain.Template) error {
	return nil
}
func (p *blockingRetryPorts) ResumeFiles(context.Context, domain.Cell, domain.Template) error {
	return nil
}
func (p *blockingRetryPorts) CreateContainers(context.Context, domain.Cell, domain.Template) error {
	return nil
}
func (p *blockingRetryPorts) CleanContainers(context.Context, domain.Cell) error { return nil }
func (p *blockingRetryPorts) CleanSession(context.Context, domain.Cell) error    { return nil }
func (p *blockingRetryPorts) CreateSession(ctx context.Context, cell domain.Cell) error {
	p.mu.Lock()
	p.creates[cell.Name]++
	p.mu.Unlock()
	p.entered <- cell.Name
	select {
	case <-p.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (p *blockingRetryPorts) PrepareSession(context.Context, domain.Cell) error { return nil }
func (p *blockingRetryPorts) UpdateStatusLabel(context.Context, domain.Cell) error {
	return nil
}
func (p *blockingRetryPorts) EnterSession(context.Context, domain.Cell) error { return nil }
func (p *blockingRetryPorts) EnterRootSession(context.Context, domain.ProjectConfig) error {
	return nil
}
func (p *blockingRetryPorts) ExitSession(context.Context) error { return nil }
func (p *blockingRetryPorts) createCount(cell string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.creates[cell]
}

func newConcurrentRetryUseCase(store state.SQLiteCellStateAdapter, ports *blockingRetryPorts, attempt string) RetryCellUseCase {
	return RetryCellUseCase{
		Config:           ports,
		State:            store,
		CellFactory:      ports,
		SourceFactory:    ports,
		Files:            ports,
		ContainerFactory: ports,
		SessionFactory:   ports,
		IDs:              fixedIDGenerator{id: attempt},
	}
}
