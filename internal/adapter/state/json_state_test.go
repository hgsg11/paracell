package state

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/shige1114/paradev/internal/domain"
)

func TestStateが存在しない場合は空のCell一覧を返す(t *testing.T) {
	store := JSONCellStateAdapter{Path: filepath.Join(t.TempDir(), ".pdev", "state.json")}

	cells, err := store.LoadCells(context.Background())

	if err != nil {
		t.Fatalf("空state読み込みでエラーが返った: %v", err)
	}
	if len(cells) != 0 {
		t.Fatalf("cells length = %d, want 0", len(cells))
	}
}

func TestCellを保存して読み戻せる(t *testing.T) {
	store := JSONCellStateAdapter{Path: filepath.Join(t.TempDir(), ".pdev", "state.json")}
	cell := domain.Cell{ID: "cell-1", Issue: "123", Name: "123", Template: "webapp"}
	if err := cell.MarkDone(); err != nil {
		t.Fatalf("Cellをdoneにできなかった: %v", err)
	}

	if err := store.SaveCells(context.Background(), []domain.Cell{cell}); err != nil {
		t.Fatalf("state保存でエラーが返った: %v", err)
	}
	cells, err := store.LoadCells(context.Background())

	if err != nil {
		t.Fatalf("state読み込みでエラーが返った: %v", err)
	}
	if len(cells) != 1 {
		t.Fatalf("cells length = %d, want 1", len(cells))
	}
	if cells[0].ID != "cell-1" {
		t.Fatalf("cell ID = %q, want %q", cells[0].ID, "cell-1")
	}
	if !cells[0].IsDone() {
		t.Fatal("IsDone = false, want true")
	}
}
