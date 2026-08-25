package usecase

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"text/template"

	"github.com/hgsg11/paracell/internal/domain"
)

func TestForkCellはCellを作成して外部リソースを順番に作る(t *testing.T) {
	ctx := context.Background()
	ports := newFakePorts()
	uc := ForkCellUseCase{
		Config:           ports,
		State:            ports,
		CellFactory:      ports,
		SourceFactory:    ports,
		ContainerFactory: ports,
		SessionFactory:   ports,
		Files:            ports,
		IDs:              fixedIDGenerator{id: "cell-1"},
	}

	cell, err := uc.Execute(ctx, ForkCellInput{Issue: "123", Template: "webapp", Command: "review the API"})
	if err != nil {
		t.Fatalf("ForkCellでエラーが返った: %v", err)
	}
	if cell.ID != "cell-1" {
		t.Fatalf("cell ID = %q, want %q", cell.ID, "cell-1")
	}
	if cell.IsDone() {
		t.Fatal("IsDone = true, want false")
	}
	wantCalls := []string{
		"factory:source:git",
		"factory:container:docker",
		"factory:session:tmux",
		"state:save:1",
		"source:fork:123",
		"state:save:1",
		"files:copy:123:.env,apps/web/.env.local",
		"state:save:1",
		"containers:fork:123",
		"state:save:1",
		"session:fork:123:nvim 123; codex review the API",
		"state:save:1",
	}
	if !reflect.DeepEqual(ports.calls, wantCalls) {
		t.Fatalf("呼び出し順 = %#v, want %#v", ports.calls, wantCalls)
	}
}

func TestForkCellはNoteを外部Resource作成前に正規化検証する(t *testing.T) {
	valid := "  PostgreSQL\t案\n"
	ports := newFakePorts()
	cell, err := newForkCellUseCase(ports).Execute(context.Background(), ForkCellInput{Issue: "123", Template: "webapp", Note: &valid})
	if err != nil {
		t.Fatal(err)
	}
	if cell.Note != "PostgreSQL 案" || len(ports.cells) != 1 || ports.cells[0].Note != "PostgreSQL 案" {
		t.Fatalf("cell = %#v, stored = %#v", cell, ports.cells)
	}
	if cell.Branch != "feat/123" || cell.Source.Path != ".paracell/cells/123/source" || cell.Session.Name != "myapp-123" || cell.Containers.Network != "paracell-myapp-123" {
		t.Fatalf("note changed managed resource identifiers: %#v", cell)
	}

	for _, invalid := range []string{"", strings.Repeat("案", 21)} {
		ports := newFakePorts()
		_, err := newForkCellUseCase(ports).Execute(context.Background(), ForkCellInput{Issue: "123", Template: "webapp", Note: &invalid})
		if err == nil {
			t.Fatalf("note %q returned no error", invalid)
		}
		if len(ports.calls) != 0 || len(ports.cells) != 0 {
			t.Fatalf("invalid note created resources or state: calls=%#v cells=%#v", ports.calls, ports.cells)
		}
	}
}

func TestForkCellは各工程のCheckpointを保存して失敗したCellと完了工程を保持する(t *testing.T) {
	originalErr := errors.New("creation failed")
	tests := []struct {
		name          string
		configure     func(*fakePorts)
		wantStage     domain.CreationStage
		wantCompleted []domain.CreationStage
	}{
		{
			name: "source作成",
			configure: func(ports *fakePorts) {
				ports.createSourceErr = originalErr
			},
			wantStage: domain.CreationStageSource,
		},
		{
			name: "fileコピー",
			configure: func(ports *fakePorts) {
				ports.copyFilesErr = originalErr
			},
			wantStage:     domain.CreationStageFiles,
			wantCompleted: []domain.CreationStage{domain.CreationStageSource},
		},
		{
			name: "container作成",
			configure: func(ports *fakePorts) {
				ports.createContainersErr = originalErr
			},
			wantStage:     domain.CreationStageContainers,
			wantCompleted: []domain.CreationStage{domain.CreationStageSource, domain.CreationStageFiles},
		},
		{
			name: "session作成",
			configure: func(ports *fakePorts) {
				ports.createSessionErr = originalErr
			},
			wantStage: domain.CreationStageSession,
			wantCompleted: []domain.CreationStage{
				domain.CreationStageSource, domain.CreationStageFiles, domain.CreationStageContainers,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ports := newFakePorts()
			tt.configure(ports)
			uc := newForkCellUseCase(ports)

			_, err := uc.Execute(context.Background(), ForkCellInput{Issue: "123", Template: "webapp"})

			if !errors.Is(err, originalErr) {
				t.Fatalf("error = %v, want original error", err)
			}
			if len(ports.cells) != 1 {
				t.Fatalf("失敗後のcells = %#v, want one failed cell", ports.cells)
			}
			failed := ports.cells[0]
			if failed.CreationStatus() != domain.CreationFailed || failed.Creation.FailedStage != tt.wantStage || failed.Creation.LastError != originalErr.Error() {
				t.Fatalf("creation = %#v", failed.Creation)
			}
			if !reflect.DeepEqual(failed.Creation.CompletedStages, tt.wantCompleted) {
				t.Fatalf("completed stages = %#v, want %#v", failed.Creation.CompletedStages, tt.wantCompleted)
			}
			for _, call := range ports.calls {
				if strings.Contains(call, "clean") || strings.Contains(call, "rollback") {
					t.Fatalf("完了済みresourceをcleanupした: %#v", ports.calls)
				}
			}
		})
	}
}

func TestForkCellはSharedDatabase接続後のSession失敗をRollbackしてRetryする(t *testing.T) {
	ports := newFakePorts()
	tpl := ports.config.Templates["webapp"]
	tpl.Containers.Services["db"] = domain.ContainerServiceTemplate{
		SourceContainer: "myapp-db",
		Database:        &domain.DatabaseConfig{Mode: domain.DatabaseModeShared},
	}
	ports.config.Templates["webapp"] = tpl
	ports.createSessionErr = errors.New("tmux failed")

	_, err := newForkCellUseCase(ports).Execute(context.Background(), ForkCellInput{Issue: "123", Template: "webapp"})
	if err == nil {
		t.Fatal("session failure was not returned")
	}
	failed := ports.cells[0]
	if failed.CreationStageCompleted(domain.CreationStageContainers) {
		t.Fatalf("container checkpoint remained after shared database rollback: %#v", failed.Creation.CompletedStages)
	}
	if !containsCall(ports.calls, "containers:clean:123") {
		t.Fatalf("shared database containers were not rolled back: %#v", ports.calls)
	}

	ports.createSessionErr = nil
	ports.calls = nil
	cell, err := newRetryCellUseCase(ports).Execute(context.Background(), RetryCellInput{Cell: "123"})
	if err != nil {
		t.Fatalf("retry failed: %v", err)
	}
	if cell.CreationStatus() != domain.CreationReady {
		t.Fatalf("creation status = %q, want ready", cell.CreationStatus())
	}
	wantOrder := []string{"containers:clean:123", "containers:fork:123", "session:fork:123:nvim 123; codex "}
	position := 0
	for _, call := range ports.calls {
		if position < len(wantOrder) && call == wantOrder[position] {
			position++
		}
	}
	if position != len(wantOrder) {
		t.Fatalf("retry did not recreate containers before session: calls=%#v", ports.calls)
	}
}

func containsCall(calls []string, want string) bool {
	for _, call := range calls {
		if call == want {
			return true
		}
	}
	return false
}

func TestForkCellは初回State登録失敗時にResourceもFailedCellも作らない(t *testing.T) {
	originalErr := errors.New("state write failed")
	ports := newFakePorts()
	ports.updateCellsErr = originalErr

	_, err := newForkCellUseCase(ports).Execute(context.Background(), ForkCellInput{Issue: "123", Template: "webapp"})

	if !errors.Is(err, originalErr) || len(ports.cells) != 0 {
		t.Fatalf("error = %v, cells = %#v", err, ports.cells)
	}
	for _, call := range ports.calls {
		if strings.Contains(call, ":fork:") {
			t.Fatalf("resource was created: %#v", ports.calls)
		}
	}
}

func TestForkCellは元の工程Errorを保持してFailed保存Errorを結合する(t *testing.T) {
	createErr := errors.New("copy failed")
	saveErr := errors.New("failed state unavailable")
	ports := newFakePorts()
	ports.copyFilesErr = createErr
	ports.updateCellsErrors = []error{nil, nil, saveErr}

	_, err := newForkCellUseCase(ports).Execute(context.Background(), ForkCellInput{Issue: "123", Template: "webapp"})
	if !errors.Is(err, createErr) || !errors.Is(err, saveErr) {
		t.Fatalf("error = %v, want both errors", err)
	}
}

func TestForkCellはCheckpoint保存失敗でもSourceを保持してFailedとして再開可能にする(t *testing.T) {
	saveErr := errors.New("checkpoint unavailable")
	ports := newFakePorts()
	ports.updateCellsErrors = []error{nil, saveErr, nil}

	_, err := newForkCellUseCase(ports).Execute(context.Background(), ForkCellInput{Issue: "123", Template: "webapp"})
	if !errors.Is(err, saveErr) {
		t.Fatalf("error = %v", err)
	}
	if len(ports.cells) != 1 || ports.cells[0].Creation.FailedStage != domain.CreationStageSource {
		t.Fatalf("cells = %#v", ports.cells)
	}
	for _, call := range ports.calls {
		if strings.Contains(call, "source:clean") || strings.Contains(call, "rollback") {
			t.Fatalf("source was removed: %#v", ports.calls)
		}
	}
}

func newForkCellUseCase(ports *fakePorts) ForkCellUseCase {
	return ForkCellUseCase{
		Config:           ports,
		State:            ports,
		CellFactory:      ports,
		SourceFactory:    ports,
		ContainerFactory: ports,
		SessionFactory:   ports,
		Files:            ports,
		IDs:              fixedIDGenerator{id: "cell-1"},
	}
}

func newRetryCellUseCase(ports *fakePorts) RetryCellUseCase {
	return RetryCellUseCase{
		Config:           ports,
		State:            ports,
		CellFactory:      ports,
		SourceFactory:    ports,
		ContainerFactory: ports,
		SessionFactory:   ports,
		Files:            ports,
		IDs:              fixedIDGenerator{id: "attempt-1"},
	}
}

func TestRetryCellはIDIssueNameで失敗工程から最新Templateを使って再開する(t *testing.T) {
	for _, identifier := range []string{"cell-1", "123", "name"} {
		t.Run(identifier, func(t *testing.T) {
			ports := newFakePorts()
			ports.createContainersErr = errors.New("docker failed")
			note := "PostgreSQL案"
			_, err := newForkCellUseCase(ports).Execute(context.Background(), ForkCellInput{
				Issue: "123", Template: "webapp", Command: "original command", Note: &note,
			})
			if err == nil {
				t.Fatal("fork succeeded")
			}
			original := ports.cells[0]
			if identifier == "name" {
				identifier = original.Name
			}
			ports.createContainersErr = nil
			updated := ports.config.Templates["webapp"]
			updated.Session.Windows[0].Command = "latest {{.name}} {{.Command}}"
			updated.Containers.Services["web"] = domain.ContainerServiceTemplate{
				SourceContainer: "myapp-web-v2",
				Environment:     map[string]string{"RETRY": "true"},
			}
			ports.config.Templates["webapp"] = updated
			ports.calls = nil

			cell, err := newRetryCellUseCase(ports).Execute(context.Background(), RetryCellInput{Cell: identifier})
			if err != nil {
				t.Fatalf("RetryCell error: %v", err)
			}
			if cell.CreationStatus() != domain.CreationReady || cell.Creation.FailedStage != "" || cell.Creation.LastError != "" {
				t.Fatalf("creation = %#v", cell.Creation)
			}
			if cell.ID != original.ID || cell.Branch != original.Branch || cell.Source.Path != original.Source.Path || cell.Containers.Services["web"].ContainerName != original.Containers.Services["web"].ContainerName {
				t.Fatalf("identifiers changed: before=%#v after=%#v", original, cell)
			}
			if cell.Note != note {
				t.Fatalf("note = %q, want %q", cell.Note, note)
			}
			if got := cell.Session.Windows[0].Command; got != "latest 123 original command" {
				t.Fatalf("latest rendered command = %q", got)
			}
			for _, forbidden := range []string{"source:resume", "files:copy"} {
				for _, call := range ports.calls {
					if strings.Contains(call, forbidden) {
						t.Fatalf("completed stage reran: %#v", ports.calls)
					}
				}
			}
		})
	}
}

func TestRetryCellは再失敗情報を更新してさらにRetryできる(t *testing.T) {
	ports := newFakePorts()
	ports.createSessionErr = errors.New("first tmux failure")
	_, _ = newForkCellUseCase(ports).Execute(context.Background(), ForkCellInput{Issue: "123", Template: "webapp"})

	ports.createSessionErr = errors.New("second tmux failure")
	_, err := newRetryCellUseCase(ports).Execute(context.Background(), RetryCellInput{Cell: "123"})
	if err == nil || ports.cells[0].Creation.LastError != "second tmux failure" || ports.cells[0].Creation.FailedStage != domain.CreationStageSession {
		t.Fatalf("retry failure: err=%v creation=%#v", err, ports.cells[0].Creation)
	}

	ports.createSessionErr = nil
	if _, err := newRetryCellUseCase(ports).Execute(context.Background(), RetryCellInput{Cell: "123"}); err != nil {
		t.Fatalf("second retry failed: %v", err)
	}
	if ports.cells[0].CreationStatus() != domain.CreationReady {
		t.Fatalf("creation = %#v", ports.cells[0].Creation)
	}
}

func TestRetryCellはSource失敗からResume用Adapter処理で全工程を進める(t *testing.T) {
	ports := newFakePorts()
	ports.createSourceErr = errors.New("worktree failed")
	_, _ = newForkCellUseCase(ports).Execute(context.Background(), ForkCellInput{Issue: "123", Template: "webapp"})
	ports.createSourceErr = nil
	ports.calls = nil

	if _, err := newRetryCellUseCase(ports).Execute(context.Background(), RetryCellInput{Cell: "123"}); err != nil {
		t.Fatalf("retry failed: %v", err)
	}
	wantStages := []string{"source:resume:123", "files:resume:123:.env,apps/web/.env.local", "containers:fork:123", "session:fork:123"}
	for _, want := range wantStages {
		found := false
		for _, call := range ports.calls {
			if strings.HasPrefix(call, want) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing %q in %#v", want, ports.calls)
		}
	}
}

func TestRetryCellは存在しないCellとFailed以外を拒否する(t *testing.T) {
	ports := newFakePorts()
	ports.cells = []domain.Cell{{ID: "ready", Issue: "123", Name: "123"}}
	for _, identifier := range []string{"missing", "123"} {
		if _, err := newRetryCellUseCase(ports).Execute(context.Background(), RetryCellInput{Cell: identifier}); err == nil {
			t.Fatalf("retry %q succeeded", identifier)
		}
	}
}

func TestResolveCellはIDIssueNameのいずれでも解決する(t *testing.T) {
	want := domain.Cell{ID: "cell-id", Issue: "73", Name: "fix-73"}
	for _, identifier := range []string{want.ID, want.Issue, want.Name} {
		got, ok := resolveCell([]domain.Cell{want}, identifier)
		if !ok || !reflect.DeepEqual(got, want) {
			t.Fatalf("resolve %q = %#v, %t", identifier, got, ok)
		}
	}
}

func TestForkCellは同じIssueがある場合に失敗する(t *testing.T) {
	ctx := context.Background()
	ports := newFakePorts()
	ports.cells = []domain.Cell{{ID: "existing", Issue: "123", Name: "123"}}
	uc := ForkCellUseCase{
		Config:           ports,
		State:            ports,
		CellFactory:      ports,
		SourceFactory:    ports,
		ContainerFactory: ports,
		SessionFactory:   ports,
		Files:            ports,
		IDs:              fixedIDGenerator{id: "cell-1"},
	}

	_, err := uc.Execute(ctx, ForkCellInput{Issue: "123", Template: "webapp"})

	if err == nil {
		t.Fatal("同じIssueなのにエラーが返らなかった")
	}
	if len(ports.calls) != 0 {
		t.Fatalf("外部リソースが作成された: %#v", ports.calls)
	}
}

func TestForkCellはFailedCellの重複を上書きせずRetryを案内する(t *testing.T) {
	ports := newFakePorts()
	failed := domain.Cell{ID: "existing", Issue: "123", Name: "123"}
	failed.BeginCreation("command")
	failed.FailCreation(domain.CreationStageFiles, errors.New("copy failed"))
	ports.cells = []domain.Cell{failed}

	_, err := newForkCellUseCase(ports).Execute(context.Background(), ForkCellInput{Issue: "123", Template: "webapp"})
	if err == nil || !strings.Contains(err.Error(), "paracell retry 123") {
		t.Fatalf("error = %v", err)
	}
	if !reflect.DeepEqual(ports.cells, []domain.Cell{failed}) {
		t.Fatalf("failed cell changed: %#v", ports.cells)
	}
}

func TestForkCellはEnvironmentTemplateError時にResourceを作成しない(t *testing.T) {
	ctx := context.Background()
	ports := newFakePorts()
	ports.configErr = errors.New(`render environment "APP_ENV" for service "web": unknown variable`)
	uc := ForkCellUseCase{
		Config:           ports,
		State:            ports,
		CellFactory:      ports,
		SourceFactory:    ports,
		ContainerFactory: ports,
		SessionFactory:   ports,
		Files:            ports,
		IDs:              fixedIDGenerator{id: "cell-1"},
	}

	_, err := uc.Execute(ctx, ForkCellInput{Issue: "123", Template: "webapp"})
	if err == nil {
		t.Fatal("environment template errorが返らなかった")
	}
	if len(ports.calls) != 0 {
		t.Fatalf("template validation後にresourceが作成された: %#v", ports.calls)
	}
}

func TestForkCellはProvider解決失敗時にResourceもFailedCellも作らない(t *testing.T) {
	for _, configure := range []func(*fakePorts){
		func(ports *fakePorts) { ports.sourceFactoryErr = errors.New("source provider failed") },
		func(ports *fakePorts) { ports.containerFactoryErr = errors.New("container provider failed") },
		func(ports *fakePorts) { ports.sessionFactoryErr = errors.New("session provider failed") },
	} {
		ports := newFakePorts()
		configure(ports)
		_, err := newForkCellUseCase(ports).Execute(context.Background(), ForkCellInput{Issue: "123", Template: "webapp"})
		if err == nil || len(ports.cells) != 0 {
			t.Fatalf("error=%v cells=%#v", err, ports.cells)
		}
		for _, call := range ports.calls {
			if strings.Contains(call, ":fork:") {
				t.Fatalf("resource was created: %#v", ports.calls)
			}
		}
	}
}

func TestForkCellはCell生成失敗時にResourceもFailedCellも作らない(t *testing.T) {
	ports := newFakePorts()
	ports.newCellErr = errors.New("cell generation failed")
	_, err := newForkCellUseCase(ports).Execute(context.Background(), ForkCellInput{Issue: "123", Template: "webapp"})
	if !errors.Is(err, ports.newCellErr) || len(ports.cells) != 0 || len(ports.calls) != 0 {
		t.Fatalf("error=%v cells=%#v calls=%#v", err, ports.cells, ports.calls)
	}
}

func TestCleanCellはCellを削除してStateから消す(t *testing.T) {
	ctx := context.Background()
	ports := newFakePorts()
	ports.cells = []domain.Cell{
		func() domain.Cell {
			cell := domain.Cell{ID: "cell-1", Issue: "123", Name: "123"}
			if err := cell.MarkDone(); err != nil {
				t.Fatalf("Cellをdoneにできなかった: %v", err)
			}
			return cell
		}(),
		{ID: "cell-2", Issue: "456", Name: "456"},
	}
	uc := CleanCellUseCase{
		Config:           ports,
		State:            ports,
		SourceFactory:    ports,
		ContainerFactory: ports,
		SessionFactory:   ports,
	}

	err := uc.Execute(ctx, CleanCellInput{Cell: "123"})
	if err != nil {
		t.Fatalf("CleanCellでエラーが返った: %v", err)
	}
	wantCalls := []string{
		"factory:session:tmux",
		"factory:container:docker",
		"factory:source:git",
		"session:clean:123",
		"containers:clean:123",
		"source:clean:123",
		"state:save:1",
	}
	if !reflect.DeepEqual(ports.calls, wantCalls) {
		t.Fatalf("呼び出し順 = %#v, want %#v", ports.calls, wantCalls)
	}
	if ports.cells[0].Issue != "456" {
		t.Fatalf("残ったcell issue = %q, want %q", ports.cells[0].Issue, "456")
	}
}

func TestCleanCellはDoneでないCellをCleanしない(t *testing.T) {
	ctx := context.Background()
	ports := newFakePorts()
	ports.cells = []domain.Cell{
		{ID: "cell-1", Issue: "123", Name: "123"},
	}
	uc := CleanCellUseCase{
		Config:           ports,
		State:            ports,
		SourceFactory:    ports,
		ContainerFactory: ports,
		SessionFactory:   ports,
	}

	err := uc.Execute(ctx, CleanCellInput{Cell: "123"})

	if err == nil {
		t.Fatal("doneでないcellなのに削除できてしまった")
	}
	if err.Error() != "完了済みではないので消せない" {
		t.Fatalf("error = %q, want %q", err.Error(), "完了済みではないので消せない")
	}
}

func TestCleanCellは削除対象が既に無ければ他も消してStateから消す(t *testing.T) {
	ctx := context.Background()
	ports := newFakePorts()
	ports.cleanSessionErr = domain.ErrNotFound
	ports.cleanContainersErr = domain.ErrNotFound
	ports.cleanSourceErr = domain.ErrNotFound
	ports.cells = []domain.Cell{
		func() domain.Cell {
			cell := domain.Cell{ID: "cell-1", Issue: "123", Name: "123"}
			if err := cell.MarkDone(); err != nil {
				t.Fatalf("Cellをdoneにできなかった: %v", err)
			}
			return cell
		}(),
	}
	uc := CleanCellUseCase{
		Config:           ports,
		State:            ports,
		SourceFactory:    ports,
		ContainerFactory: ports,
		SessionFactory:   ports,
	}

	err := uc.Execute(ctx, CleanCellInput{Cell: "123"})
	if err != nil {
		t.Fatalf("CleanCellでエラーが返った: %v", err)
	}
	wantCalls := []string{
		"factory:session:tmux",
		"factory:container:docker",
		"factory:source:git",
		"session:clean:123",
		"containers:clean:123",
		"source:clean:123",
		"state:save:0",
	}
	if !reflect.DeepEqual(ports.calls, wantCalls) {
		t.Fatalf("呼び出し順 = %#v, want %#v", ports.calls, wantCalls)
	}
}

func TestCleanCellは削除エラーなら途中で止める(t *testing.T) {
	ctx := context.Background()
	ports := newFakePorts()
	ports.cleanSessionErr = errors.New("tmux permission denied")
	ports.cells = []domain.Cell{
		func() domain.Cell {
			cell := domain.Cell{ID: "cell-1", Issue: "123", Name: "123"}
			if err := cell.MarkDone(); err != nil {
				t.Fatalf("Cellをdoneにできなかった: %v", err)
			}
			return cell
		}(),
	}
	uc := CleanCellUseCase{
		Config:           ports,
		State:            ports,
		SourceFactory:    ports,
		ContainerFactory: ports,
		SessionFactory:   ports,
	}

	err := uc.Execute(ctx, CleanCellInput{Cell: "123"})
	if err == nil {
		t.Fatal("削除エラーなのに成功した")
	}
	if err.Error() != "tmux permission denied" {
		t.Fatalf("error = %q, want %q", err.Error(), "tmux permission denied")
	}
	wantCalls := []string{
		"factory:session:tmux",
		"factory:container:docker",
		"factory:source:git",
		"session:clean:123",
	}
	if !reflect.DeepEqual(ports.calls, wantCalls) {
		t.Fatalf("呼び出し順 = %#v, want %#v", ports.calls, wantCalls)
	}
}

func TestMarkCellDoneはStateのCellのDoneを切り替える(t *testing.T) {
	ctx := context.Background()
	ports := newFakePorts()
	ports.cells = []domain.Cell{
		{ID: "cell-1", Issue: "123", Name: "123"},
	}

	uc := MarkCellDoneUseCase{State: ports}
	cell, err := uc.Execute(ctx, MarkCellDoneInput{Cell: "123"})
	if err != nil {
		t.Fatalf("MarkCellDoneでエラーが返った: %v", err)
	}
	if !cell.IsDone() {
		t.Fatal("IsDone = false, want true")
	}
	if !ports.cells[0].IsDone() {
		t.Fatal("stateのcellがdoneになっていない")
	}

	cell, err = uc.Execute(ctx, MarkCellDoneInput{Cell: "123"})
	if err != nil {
		t.Fatalf("MarkCellDoneの解除でエラーが返った: %v", err)
	}
	if cell.IsDone() {
		t.Fatal("IsDone = true, want false")
	}
	if ports.cells[0].IsDone() {
		t.Fatal("stateのcellがdoneのままになっている")
	}
}

func TestSetCellStatusはStateのCellのStatusを更新する(t *testing.T) {
	ctx := context.Background()
	ports := newFakePorts()
	ports.cells = []domain.Cell{
		{ID: "cell-1", Issue: "123", Name: "123"},
	}

	uc := SetCellStatusUseCase{State: ports}
	cell, err := uc.Execute(ctx, SetCellStatusInput{Cell: "123", Status: domain.Ready})
	if err != nil {
		t.Fatalf("SetCellStatusでエラーが返った: %v", err)
	}
	if got := cell.Status(); got != domain.Ready {
		t.Fatalf("Status = %q, want %q", got, domain.Ready)
	}
	if got := ports.cells[0].Status(); got != domain.Ready {
		t.Fatalf("stateのcell status = %q, want %q", got, domain.Ready)
	}
}

func TestListCellsはStateのCell一覧を返す(t *testing.T) {
	ctx := context.Background()
	ports := newFakePorts()
	ports.cells = []domain.Cell{
		{ID: "cell-1", Issue: "123", Name: "123", Template: "default"},
		{ID: "cell-2", Issue: "456", Name: "456", Template: "webapp"},
	}
	uc := ListCellsUseCase{State: ports}

	cells, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("ListCellsでエラーが返った: %v", err)
	}
	if !reflect.DeepEqual(cells, ports.cells) {
		t.Fatalf("cells = %#v, want %#v", cells, ports.cells)
	}
}

func TestCreateCellはDBCopy設定をCellへ保持する(t *testing.T) {
	ctx := context.Background()
	ports := newFakePorts()
	ports.config.Templates["dbapp"] = domain.Template{
		Name: "dbapp",
		Repository: domain.RepositoryTemplate{
			BranchPrefix: "feat/",
			Base:         "main",
		},
		Containers: domain.ContainerTemplate{
			Services: map[string]domain.ContainerServiceTemplate{
				"db": {
					SourceContainer: "myapp-db",
					VolumeMode:      "copy",
					Database: &domain.DatabaseConfig{
						Mode:      domain.DatabaseModeCopy,
						System:    "mysql",
						CopyMode:  "schema",
						InitFiles: []string{"docker/mysql/init/001-users.sql"},
					},
				},
			},
		},
		Session: domain.SessionTemplate{Windows: []domain.SessionWindowTemplate{}},
	}
	uc := ForkCellUseCase{
		Config:           ports,
		State:            ports,
		CellFactory:      ports,
		SourceFactory:    ports,
		ContainerFactory: ports,
		SessionFactory:   ports,
		Files:            ports,
		IDs:              fixedIDGenerator{id: "cell-1"},
	}

	cell, err := uc.Execute(ctx, ForkCellInput{Issue: "124", Template: "dbapp"})
	if err != nil {
		t.Fatalf("ForkCellでエラーが返った: %v", err)
	}

	service := cell.Containers.Services["db"]
	if service.SourceContainer != "myapp-db" {
		t.Fatalf("SourceContainer = %q, want %q", service.SourceContainer, "myapp-db")
	}
	if service.VolumeMode != "copy" {
		t.Fatalf("VolumeMode = %q, want %q", service.VolumeMode, "copy")
	}
	if service.Database == nil {
		t.Fatal("Database = nil, want non-nil")
	}
	if service.Database.System != "mysql" {
		t.Fatalf("Database.System = %q, want %q", service.Database.System, "mysql")
	}
	if service.Database.Mode != domain.DatabaseModeCopy {
		t.Fatalf("Database.Mode = %q, want %q", service.Database.Mode, domain.DatabaseModeCopy)
	}
	if service.Database.CopyMode != "schema" {
		t.Fatalf("Database.CopyMode = %q, want %q", service.Database.CopyMode, "schema")
	}
	if !reflect.DeepEqual(service.Database.InitFiles, []string{"docker/mysql/init/001-users.sql"}) {
		t.Fatalf("Database.InitFiles = %#v, want %#v", service.Database.InitFiles, []string{"docker/mysql/init/001-users.sql"})
	}
	wantFileCopy := "files:copy:124:docker/mysql/init/001-users.sql"
	foundFileCopy := false
	for _, call := range ports.calls {
		if call == wantFileCopy {
			foundFileCopy = true
			break
		}
	}
	if !foundFileCopy {
		t.Fatalf("files copy calls = %#v, want %q", ports.calls, wantFileCopy)
	}
}

func TestCreateCellはVolumeModeをCellへ保持する(t *testing.T) {
	ctx := context.Background()
	ports := newFakePorts()
	ports.config.Templates["volumeapp"] = domain.Template{
		Name: "volumeapp",
		Repository: domain.RepositoryTemplate{
			BranchPrefix: "feat/",
			Base:         "main",
		},
		Containers: domain.ContainerTemplate{
			Services: map[string]domain.ContainerServiceTemplate{
				"web": {
					SourceContainer: "myapp-web",
					VolumeMode:      "copy",
				},
			},
		},
		Session: domain.SessionTemplate{Windows: []domain.SessionWindowTemplate{}},
	}
	uc := ForkCellUseCase{
		Config:           ports,
		State:            ports,
		CellFactory:      ports,
		SourceFactory:    ports,
		ContainerFactory: ports,
		SessionFactory:   ports,
		Files:            ports,
		IDs:              fixedIDGenerator{id: "cell-1"},
	}

	cell, err := uc.Execute(ctx, ForkCellInput{Issue: "125", Template: "volumeapp"})
	if err != nil {
		t.Fatalf("ForkCellでエラーが返った: %v", err)
	}
	if got := cell.Containers.Services["web"].VolumeMode; got != "copy" {
		t.Fatalf("VolumeMode = %q, want %q", got, "copy")
	}
}

func TestCreateCellはRepositoryBaseをCellへ保持する(t *testing.T) {
	ctx := context.Background()
	ports := newFakePorts()
	ports.config.Templates["baseapp"] = domain.Template{
		Name: "baseapp",
		Repository: domain.RepositoryTemplate{
			BranchPrefix: "feat/",
			Base:         "current",
		},
		Session: domain.SessionTemplate{Windows: []domain.SessionWindowTemplate{}},
	}
	uc := ForkCellUseCase{
		Config:           ports,
		State:            ports,
		CellFactory:      ports,
		SourceFactory:    ports,
		ContainerFactory: ports,
		SessionFactory:   ports,
		Files:            ports,
		IDs:              fixedIDGenerator{id: "cell-1"},
	}

	cell, err := uc.Execute(ctx, ForkCellInput{Issue: "126", Template: "baseapp"})
	if err != nil {
		t.Fatalf("ForkCellでエラーが返った: %v", err)
	}
	if cell.Base != "current" {
		t.Fatalf("cell base = %q, want %q", cell.Base, "current")
	}
}

type fakePorts struct {
	config               domain.Config
	configErr            error
	newCellErr           error
	sourceFactoryErr     error
	containerFactoryErr  error
	sessionFactoryErr    error
	cells                []domain.Cell
	calls                []string
	createSourceErr      error
	resumeSourceErr      error
	copyFilesErr         error
	createContainersErr  error
	createSessionErr     error
	updateCellsErr       error
	updateCellsErrors    []error
	sourceCreation       SourceCreation
	cleanSourceErr       error
	cleanContainersErr   error
	cleanSessionErr      error
	updateStatusLabelErr error
}

func newFakePorts() *fakePorts {
	return &fakePorts{
		config: domain.Config{
			Project: domain.ProjectConfig{Name: "myapp"},
			Providers: domain.ProviderConfig{
				Source:    "git",
				Container: "docker",
				Session:   "tmux",
			},
			Templates: map[string]domain.Template{
				"webapp": {
					Name: "webapp",
					Repository: domain.RepositoryTemplate{
						BranchPrefix: "feat/",
						Base:         "main",
					},
					Files: []string{".env", "apps/web/.env.local"},
					Containers: domain.ContainerTemplate{
						Services: map[string]domain.ContainerServiceTemplate{
							"web": {SourceContainer: "myapp-web"},
						},
					},
					Session: domain.SessionTemplate{
						Windows: []domain.SessionWindowTemplate{{Name: "editor", Command: "nvim {{.issue}}; codex {{.Command}}"}},
					},
				},
			},
		},
	}
}

func TestForkCellはAbstractTemplateを選択できない(t *testing.T) {
	ports := newFakePorts()
	ports.config.AbstractTemplates = map[string]struct{}{"base": {}}
	uc := newForkCellUseCase(ports)

	_, err := uc.Execute(context.Background(), ForkCellInput{Issue: "77", Template: "base"})
	if err == nil {
		t.Fatal("abstract templateを選択できた")
	}
	if got, want := err.Error(), `template "base" is abstract and cannot be selected`; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
	if len(ports.calls) != 0 {
		t.Fatalf("abstract template拒否前に副作用が発生した: %#v", ports.calls)
	}
}

func (f *fakePorts) Load(ctx context.Context, vars *domain.TemplateVars) (domain.Config, error) {
	_ = ctx
	if f.configErr != nil {
		return domain.Config{}, f.configErr
	}
	if vars == nil {
		return f.config, nil
	}
	cfg := f.config
	cfg.Templates = make(map[string]domain.Template, len(f.config.Templates))
	for name, tpl := range f.config.Templates {
		rendered := tpl
		rendered.Session.Windows = make([]domain.SessionWindowTemplate, 0, len(tpl.Session.Windows))
		for _, window := range tpl.Session.Windows {
			command, err := renderString(window.Command, map[string]string{
				"issue":   vars.Issue,
				"name":    vars.Name,
				"Command": vars.Command,
			})
			if err != nil {
				return domain.Config{}, err
			}
			rendered.Session.Windows = append(rendered.Session.Windows, domain.SessionWindowTemplate{
				Name:    window.Name,
				Command: command,
			})
		}
		cfg.Templates[name] = rendered
	}
	return cfg, nil
}

func (f *fakePorts) LoadCells(ctx context.Context) ([]domain.Cell, error) {
	return append([]domain.Cell(nil), f.cells...), nil
}

func (f *fakePorts) UpdateCells(ctx context.Context, update func([]domain.Cell) ([]domain.Cell, error)) error {
	cells, err := update(append([]domain.Cell(nil), f.cells...))
	if err != nil {
		return err
	}
	f.calls = append(f.calls, "state:save:"+string(rune('0'+len(cells))))
	if len(f.updateCellsErrors) > 0 {
		err := f.updateCellsErrors[0]
		f.updateCellsErrors = f.updateCellsErrors[1:]
		if err != nil {
			return err
		}
	}
	if f.updateCellsErr != nil {
		return f.updateCellsErr
	}
	f.cells = append([]domain.Cell(nil), cells...)
	return nil
}

func (f *fakePorts) Source(provider domain.ProviderConfig) (SourcePort, error) {
	f.calls = append(f.calls, "factory:source:"+provider.Source)
	return f, f.sourceFactoryErr
}

func (f *fakePorts) Container(provider domain.ProviderConfig) (ContainerPort, error) {
	f.calls = append(f.calls, "factory:container:"+provider.Container)
	return f, f.containerFactoryErr
}

func (f *fakePorts) Session(provider domain.ProviderConfig) (SessionPort, error) {
	f.calls = append(f.calls, "factory:session:"+provider.Session)
	return f, f.sessionFactoryErr
}

func (f *fakePorts) NewCell(id string, issue string, template domain.Template, project string) (domain.Cell, error) {
	if f.newCellErr != nil {
		return domain.Cell{}, f.newCellErr
	}
	cell := domain.Cell{
		ID:       id,
		Issue:    issue,
		Name:     issue,
		Template: template.Name,
		Base:     template.Repository.Base,
		Branch:   template.Repository.BranchPrefix + issue,
		Source: domain.Source{
			Path: ".paracell/cells/" + issue + "/source",
		},
		Containers: domain.Containers{
			Network:  "paracell-" + project + "-" + issue,
			Services: map[string]domain.CellContainer{},
		},
		Session: domain.Session{
			Name: project + "-" + issue,
		},
	}
	for role, service := range template.Containers.Services {
		cell.Containers.Services[role] = domain.CellContainer{
			ContainerName:   "paracell-" + project + "-" + issue + "-" + role,
			SourceContainer: service.SourceContainer,
			VolumeMode:      service.VolumeMode,
			Database:        service.Database,
		}
	}
	for _, window := range template.Session.Windows {
		cell.Session.Windows = append(cell.Session.Windows, domain.SessionWindow{Name: window.Name, Command: window.Command})
	}
	return cell, nil
}

func (f *fakePorts) CreateSource(ctx context.Context, cell domain.Cell) (SourceCreation, error) {
	f.calls = append(f.calls, "source:fork:"+cell.Name)
	return f.sourceCreation, f.createSourceErr
}

func (f *fakePorts) ResumeSource(ctx context.Context, cell domain.Cell) error {
	f.calls = append(f.calls, "source:resume:"+cell.Name)
	return f.resumeSourceErr
}

func (f *fakePorts) CleanSource(ctx context.Context, cell domain.Cell) error {
	f.calls = append(f.calls, "source:clean:"+cell.Name)
	return f.cleanSourceErr
}

func (f *fakePorts) CopyFiles(ctx context.Context, cell domain.Cell, template domain.Template) error {
	f.calls = append(f.calls, "files:copy:"+cell.Name+":"+joinStrings(template.Files))
	return f.copyFilesErr
}

func (f *fakePorts) ResumeFiles(ctx context.Context, cell domain.Cell, template domain.Template) error {
	f.calls = append(f.calls, "files:resume:"+cell.Name+":"+joinStrings(template.Files))
	return f.copyFilesErr
}

func (f *fakePorts) CreateContainers(ctx context.Context, cell domain.Cell, template domain.Template) error {
	f.calls = append(f.calls, "containers:fork:"+cell.Name)
	return f.createContainersErr
}

func (f *fakePorts) CleanContainers(ctx context.Context, cell domain.Cell) error {
	f.calls = append(f.calls, "containers:clean:"+cell.Name)
	return f.cleanContainersErr
}

func (f *fakePorts) CreateSession(ctx context.Context, cell domain.Cell) error {
	command := ""
	if len(cell.Session.Windows) > 0 {
		command = ":" + cell.Session.Windows[0].Command
	}
	f.calls = append(f.calls, "session:fork:"+cell.Name+command)
	return f.createSessionErr
}

func (f *fakePorts) CleanSession(ctx context.Context, cell domain.Cell) error {
	f.calls = append(f.calls, "session:clean:"+cell.Name)
	return f.cleanSessionErr
}

func (f *fakePorts) EnterSession(ctx context.Context, cell domain.Cell) error {
	f.calls = append(f.calls, "session:enter:"+cell.Name)
	return nil
}

func (f *fakePorts) PrepareSession(ctx context.Context, cell domain.Cell) error {
	_ = ctx
	_ = cell
	return nil
}

func (f *fakePorts) UpdateStatusLabel(ctx context.Context, cell domain.Cell) error {
	_ = ctx
	f.calls = append(f.calls, "session:label:"+cell.DisplayLabel())
	return f.updateStatusLabelErr
}

func (f *fakePorts) EnterRootSession(ctx context.Context, project domain.ProjectConfig) error {
	_ = ctx
	f.calls = append(f.calls, "session:enter-root:"+project.Name)
	return nil
}

func (f *fakePorts) ExitSession(ctx context.Context) error {
	_ = ctx
	f.calls = append(f.calls, "session:exit")
	return nil
}

type fixedIDGenerator struct {
	id string
}

func (g fixedIDGenerator) NewID() string {
	return g.id
}

func joinStrings(values []string) string {
	if len(values) == 0 {
		return ""
	}
	out := values[0]
	for _, value := range values[1:] {
		out += "," + value
	}
	return out
}

func renderString(value string, data map[string]string) (string, error) {
	tmpl, err := template.New("value").Option("missingkey=error").Parse(value)
	if err != nil {
		return "", err
	}
	var b bytes.Buffer
	if err := tmpl.Execute(&b, data); err != nil {
		return "", err
	}
	return b.String(), nil
}
