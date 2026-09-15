package state

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/hgsg11/paracell/internal/domain"
)

func persistedTestCell(t *testing.T) domain.Cell {
	t.Helper()
	source, err := domain.NewSource(".", "main", "feat/42")
	if err != nil {
		t.Fatal(err)
	}
	sourceDriver, _ := domain.NewSourceDriverType("git")
	sessionDriver, _ := domain.NewSessionDriverType("tmux")
	container, err := domain.NewContainer([]string{"project_default"}, "db", domain.Dependency)
	if err != nil {
		t.Fatal(err)
	}
	cell, err := domain.NewCell("id", "42", "sample", "feat", domain.NewSources(sourceDriver, []domain.Source{source}), domain.NewContainers(domain.Docker, []domain.Container{container}), domain.NewSession(sessionDriver, nil), domain.NoNotification)
	if err != nil {
		t.Fatal(err)
	}
	return cell
}

func TestSQLiteStateは新規CellをVersion1で保存して読込では進めない(t *testing.T) {
	adapter := NewSQLiteCellStateAdapter(filepath.Join(t.TempDir(), "state.db"))
	cell := persistedTestCell(t)
	if err := adapter.SaveCells(context.Background(), []domain.Cell{cell}); err != nil {
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
	if first[0].Version != 1 || second[0].Version != 1 {
		t.Fatalf("versions = %d, %d", first[0].Version, second[0].Version)
	}
	if first[0].Sources.Items[0].Path != "." || first[0].Containers.Items[0].Mode != domain.Dependency {
		t.Fatalf("cell = %#v", first[0])
	}
}

func TestSQLiteStateは更新ごとにVersionを1増やし古い更新を拒否する(t *testing.T) {
	ctx := context.Background()
	adapter := NewSQLiteCellStateAdapter(filepath.Join(t.TempDir(), "state.db"))
	cell := persistedTestCell(t)
	if err := adapter.SaveCells(ctx, []domain.Cell{cell}); err != nil {
		t.Fatal(err)
	}
	changed := cell
	_ = changed.SetNote("updated")
	if err := adapter.SaveCells(ctx, []domain.Cell{changed}); err != nil {
		t.Fatal(err)
	}
	got, _ := adapter.LoadCells(ctx)
	if got[0].Version != 2 {
		t.Fatalf("version = %d", got[0].Version)
	}

	stale := cell
	_ = stale.SetNote("stale")
	if err := adapter.SaveCells(ctx, []domain.Cell{stale}); !errors.Is(err, domain.ErrVersionConflict) {
		t.Fatalf("error = %v", err)
	}
	got, _ = adapter.LoadCells(ctx)
	if got[0].Version != 2 || got[0].Note != "updated" {
		t.Fatalf("cell changed after conflict: %#v", got[0])
	}
}

func TestSQLiteStateは古いVersionによる削除を拒否する(t *testing.T) {
	ctx := context.Background()
	adapter := NewSQLiteCellStateAdapter(filepath.Join(t.TempDir(), "state.db"))
	cell := persistedTestCell(t)
	if err := adapter.SaveCells(ctx, []domain.Cell{cell}); err != nil {
		t.Fatal(err)
	}
	changed := cell
	_ = changed.SetNote("updated")
	if err := adapter.SaveCells(ctx, []domain.Cell{changed}); err != nil {
		t.Fatal(err)
	}
	if err := adapter.DeleteCell(ctx, cell); !errors.Is(err, domain.ErrVersionConflict) {
		t.Fatalf("error = %v", err)
	}
	got, _ := adapter.LoadCells(ctx)
	if len(got) != 1 {
		t.Fatal("stale delete changed state")
	}
}
