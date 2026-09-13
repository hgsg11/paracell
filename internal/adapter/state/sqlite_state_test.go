package state

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/hgsg11/paracell/internal/domain"
)

func TestSQLiteStateはVersion1のJSONをVersion2のSQLTablesへ移行する(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`CREATE TABLE cells (id TEXT PRIMARY KEY, issue TEXT NOT NULL, name TEXT NOT NULL, position INTEGER NOT NULL, data BLOB NOT NULL)`,
		`CREATE UNIQUE INDEX cells_issue_unique ON cells(issue) WHERE issue <> ''`,
		`CREATE UNIQUE INDEX cells_name_unique ON cells(name) WHERE name <> ''`,
		`PRAGMA user_version = 1`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	legacy := []byte(`{
		"id":"cell-1","issue":"123","name":"123","note":"移行中","template":"webapp",
		"base":"main","branch":"feat/123","branchMode":"reuse",
		"source":{"Path":"/tmp/cell-1"},
		"containers":{"Network":"cell-1-network","Services":{"db":{"ContainerName":"cell-1-db","SourceContainer":"app-db","VolumeMode":"copy","Database":{"mode":"copy","system":"mysql","copyMode":"schema","initFiles":["init.sql"]}}}},
		"session":{"Name":"paracell-cell-1","Windows":[{"Name":"agent","Command":"codex"}]},
		"creation":{"status":"retrying","command":"read issue","completedStages":["source","files"],"failedStage":"containers","lastError":"failed","attemptId":"attempt-1","leaseStartedAt":"2026-08-10T03:34:56Z","leaseHeartbeatAt":"2026-08-10T03:35:06Z"},
		"status":"pending","done":true
	}`)
	if _, err := db.Exec(`INSERT INTO cells(id, issue, name, position, data) VALUES (?, ?, ?, ?, ?)`, "cell-1", "123", "123", 0, legacy); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	store := SQLiteCellStateAdapter{Path: path}
	cells, err := store.LoadCells(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(cells) != 1 {
		t.Fatalf("cells length = %d, want 1", len(cells))
	}
	cell := cells[0]
	if cell.ID != "cell-1" || cell.Status() != domain.Pending || !cell.IsDone() {
		t.Fatalf("migrated cell = %#v", cell)
	}
	if cell.Containers.Services["db"].Database == nil || !reflect.DeepEqual(cell.Containers.Services["db"].Database.InitFiles, []string{"init.sql"}) {
		t.Fatalf("migrated database = %#v", cell.Containers.Services["db"].Database)
	}
	if !reflect.DeepEqual(cell.Session.Windows, []domain.SessionWindow{{Name: "agent", Command: "codex"}}) {
		t.Fatalf("migrated windows = %#v", cell.Session.Windows)
	}
	if cell.Creation.LeaseHeartbeatAt == nil || !cell.Creation.LeaseHeartbeatAt.Equal(time.Date(2026, 8, 10, 3, 35, 6, 0, time.UTC)) {
		t.Fatalf("migrated creation = %#v", cell.Creation)
	}

	db, err = sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil || version != stateSchemaVersion {
		t.Fatalf("schema version = %d, err = %v", version, err)
	}
	var status string
	var done int
	if err := db.QueryRow(`SELECT status, done FROM cells WHERE id = ?`, "cell-1").Scan(&status, &done); err != nil {
		t.Fatal(err)
	}
	if status != string(domain.Pending) || done != 1 {
		t.Fatalf("stored status = %q, done = %d", status, done)
	}
}

func TestSQLiteStateが存在しない場合は空のCell一覧を返す(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".paracell", "state.db")
	store := SQLiteCellStateAdapter{Path: path}

	cells, err := store.LoadCells(context.Background())
	if err != nil {
		t.Fatalf("空state読み込みでエラーが返った: %v", err)
	}
	if len(cells) != 0 {
		t.Fatalf("cells length = %d, want 0", len(cells))
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("state.db was not created: %v", err)
	}
}

func TestSQLiteStateはCellを保存して読み戻せる(t *testing.T) {
	store := SQLiteCellStateAdapter{Path: filepath.Join(t.TempDir(), ".paracell", "state.db")}
	cell := domain.Cell{ID: "cell-1", Issue: "123", Name: "123", Note: "API実装中", Template: "webapp"}
	if err := cell.MarkDone(); err != nil {
		t.Fatal(err)
	}
	if err := cell.SetStatus(domain.Pending); err != nil {
		t.Fatal(err)
	}

	if err := store.SaveCells(context.Background(), []domain.Cell{cell}); err != nil {
		t.Fatalf("state保存でエラーが返った: %v", err)
	}
	cells, err := store.LoadCells(context.Background())
	if err != nil {
		t.Fatalf("state読み込みでエラーが返った: %v", err)
	}
	if !reflect.DeepEqual(cells, []domain.Cell{cell}) {
		t.Fatalf("cells = %#v, want %#v", cells, []domain.Cell{cell})
	}
}

func TestSQLiteStateは未設定StatusをReadyとして読み戻せる(t *testing.T) {
	store := SQLiteCellStateAdapter{Path: filepath.Join(t.TempDir(), ".paracell", "state.db")}
	cell := domain.Cell{ID: "cell-1", Issue: "123", Name: "123", Note: "API実装中", Template: "webapp"}

	if err := store.SaveCells(context.Background(), []domain.Cell{cell}); err != nil {
		t.Fatalf("state保存でエラーが返った: %v", err)
	}
	cells, err := store.LoadCells(context.Background())
	if err != nil {
		t.Fatalf("state読み込みでエラーが返った: %v", err)
	}
	if got := cells[0].Status(); got != domain.Ready {
		t.Fatalf("Status = %q, want %q", got, domain.Ready)
	}
}

func TestSQLiteStateは作成Checkpointと失敗情報を保存して読み戻せる(t *testing.T) {
	store := SQLiteCellStateAdapter{Path: filepath.Join(t.TempDir(), ".paracell", "state.db")}
	cell := domain.Cell{ID: "cell-1", Issue: "123", Name: "123", Note: "API実装中", Template: "webapp"}
	if err := cell.SetStatus(domain.Ready); err != nil {
		t.Fatal(err)
	}
	cell.BeginCreation("fix issue")
	cell.CompleteCreationStage(domain.CreationStageSource)
	cell.CompleteCreationStage(domain.CreationStageFiles)
	cell.FailCreation(domain.CreationStageContainers, fmt.Errorf("docker failed\nport busy"))

	if err := store.SaveCells(context.Background(), []domain.Cell{cell}); err != nil {
		t.Fatal(err)
	}
	cells, err := store.LoadCells(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cells, []domain.Cell{cell}) {
		t.Fatalf("cells = %#v, want %#v", cells, []domain.Cell{cell})
	}
}

func TestSQLiteStateはDatabase設定を保存して読み戻せる(t *testing.T) {
	store := SQLiteCellStateAdapter{Path: filepath.Join(t.TempDir(), ".paracell", "state.db")}
	cell := domain.Cell{
		ID:       "cell-1",
		Issue:    "123",
		Name:     "123",
		Template: "webapp",
		Base:     "current",
		Containers: domain.Containers{
			Services: map[string]domain.CellContainer{
				"db": {
					ContainerName:   "paracell-myapp-123-db",
					SourceContainer: "myapp-db",
					Database: &domain.DatabaseConfig{
						Mode:      domain.DatabaseModeCopy,
						System:    "mysql",
						CopyMode:  "schema",
						InitFiles: []string{"docker/mysql/init/001-users.sql"},
					},
				},
				"web": {
					ContainerName:   "paracell-myapp-123-web",
					SourceContainer: "myapp-web",
					VolumeMode:      "copy",
				},
			},
		},
	}
	if err := cell.SetStatus(domain.Ready); err != nil {
		t.Fatal(err)
	}

	if err := store.SaveCells(context.Background(), []domain.Cell{cell}); err != nil {
		t.Fatalf("state保存でエラーが返った: %v", err)
	}
	cells, err := store.LoadCells(context.Background())
	if err != nil {
		t.Fatalf("state読み込みでエラーが返った: %v", err)
	}
	if !reflect.DeepEqual(cells, []domain.Cell{cell}) {
		t.Fatalf("cells = %#v, want %#v", cells, []domain.Cell{cell})
	}
}

func TestSQLiteStateは複数Adapterの更新を失わない(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".paracell", "state.db")
	stores := []SQLiteCellStateAdapter{{Path: path}, {Path: path}}
	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make(chan error, len(stores))
	for index, store := range stores {
		wg.Add(1)
		go func(index int, store SQLiteCellStateAdapter) {
			defer wg.Done()
			<-start
			errs <- store.UpdateCells(context.Background(), func(cells []domain.Cell) ([]domain.Cell, error) {
				cell := domain.Cell{ID: string(rune('a' + index)), Issue: string(rune('1' + index)), Name: string(rune('a' + index))}
				return append(cells, cell), nil
			})
		}(index, store)
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent update failed: %v", err)
		}
	}

	cells, err := stores[0].LoadCells(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(cells) != len(stores) {
		t.Fatalf("cells length = %d, want %d: %#v", len(cells), len(stores), cells)
	}
}

func TestSQLiteStateはIssueとNameの重複を拒否する(t *testing.T) {
	store := SQLiteCellStateAdapter{Path: filepath.Join(t.TempDir(), "state.db")}
	cells := []domain.Cell{
		{ID: "cell-1", Issue: "123", Name: "first"},
		{ID: "cell-2", Issue: "123", Name: "second"},
	}
	if err := store.SaveCells(context.Background(), cells); err == nil {
		t.Fatal("duplicate issue was accepted")
	}

	loaded, err := store.LoadCells(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 0 {
		t.Fatalf("failed transaction changed state: %#v", loaded)
	}
}
