package state

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/hgsg11/paracell/internal/domain"
)

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

func TestSQLiteStateはStateJSONを読み込まない(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "state.json")
	data, err := json.Marshal(map[string]any{"cells": []domain.Cell{{ID: "legacy", Issue: "1", Name: "legacy"}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, data, 0o644); err != nil {
		t.Fatal(err)
	}
	store := SQLiteCellStateAdapter{Path: filepath.Join(dir, "state.db")}

	cells, err := store.LoadCells(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(cells) != 0 {
		t.Fatalf("legacy JSON was imported: %#v", cells)
	}
	if _, err := os.Stat(legacy); err != nil {
		t.Fatalf("legacy JSON was modified: %v", err)
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
