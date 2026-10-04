package state

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/hgsg11/paracell/internal/domain"
)

func persistedTestCell(t *testing.T) domain.CommanderCell {
	t.Helper()
	sourceDriver, _ := domain.NewSourceDriverType("git")
	workspaceDriver, _ := domain.NewWorkspaceDriverType("tmux")
	group, err := domain.NewCellGroup("group-id", "42", "sample", "feat", sourceDriver, domain.Docker, domain.NoNotification)
	if err != nil {
		t.Fatal(err)
	}
	cell, err := domain.NewCommanderCell("id", &group, domain.NewWorkspace(workspaceDriver, nil))
	if err != nil {
		t.Fatal(err)
	}
	return cell
}

func TestSQLiteStateは新規CellをVersion1で保存して読込では進めない(t *testing.T) {
	adapter := NewSQLiteCellAdapter(filepath.Join(t.TempDir(), "state.db"))
	cell := persistedTestCell(t)
	if err := adapter.SaveCells(context.Background(), []domain.CommanderCell{cell}); err != nil {
		t.Fatal(err)
	}
	first, err := adapter.LoadCells(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second, err := adapter.LoadCells(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if first.Commanders[0].Version != 1 || second.Commanders[0].Version != 1 {
		t.Fatalf("versions = %d, %d", first.Commanders[0].Version, second.Commanders[0].Version)
	}
	if first.Commanders[0].Workspace.Driver != domain.Tmux {
		t.Fatalf("commander = %#v", first.Commanders[0])
	}
}

func TestSQLiteStateは更新ごとにVersionを1増やし古い更新を拒否する(t *testing.T) {
	ctx := context.Background()
	adapter := NewSQLiteCellAdapter(filepath.Join(t.TempDir(), "state.db"))
	cell := persistedTestCell(t)
	if err := adapter.SaveCells(ctx, []domain.CommanderCell{cell}); err != nil {
		t.Fatal(err)
	}
	changed := cell
	_ = changed.CellGroup.SetNote("updated")
	if err := adapter.SaveCells(ctx, []domain.CommanderCell{changed}); err != nil {
		t.Fatal(err)
	}
	got, _ := adapter.LoadCells(ctx)
	if got.Commanders[0].Version != 2 {
		t.Fatalf("version = %d", got.Commanders[0].Version)
	}

	stale := cell
	_ = stale.CellGroup.SetNote("stale")
	if err := adapter.SaveCells(ctx, []domain.CommanderCell{stale}); !errors.Is(err, domain.ErrVersionConflict) {
		t.Fatalf("error = %v", err)
	}
	got, _ = adapter.LoadCells(ctx)
	if got.Commanders[0].Version != 2 || got.Commanders[0].CellGroup.Note != "updated" {
		t.Fatalf("commander changed after conflict: %#v", got.Commanders[0])
	}
}
