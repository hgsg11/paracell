package usecase

import (
	"context"
	"strings"
	"testing"

	"github.com/hgsg11/paracell/internal/domain"
)

func TestMarkCellInReviewは実装中のCellをレビュー中にする(t *testing.T) {
	ports := newFakePorts()
	ports.cells = newUsecaseCellSet(t, [3]string{"cell-1", "123", "feat"}, [3]string{"cell-2", "456", "feat"})
	cell, err := (MarkCellInReviewUseCase{Cells: ports}).Execute(context.Background(), MarkCellInReviewInput{Cell: "456"})
	if err != nil {
		t.Fatal(err)
	}
	if cell.Progress != domain.InReview || cell.ID != "cell-2" || ports.cells.Commanders[1].Progress != domain.InReview || ports.cells.Commanders[1].Version != 2 {
		t.Fatalf("cell = %#v, stored = %#v", cell, ports.cells.Commanders)
	}
	if ports.cells.Commanders[0].Progress != domain.InProgress || ports.cells.Commanders[0].Version != 1 {
		t.Fatalf("another cell changed: %#v", ports.cells.Commanders[0])
	}
}

func TestMarkCellInProgressはレビュー中のCellを実装中に戻す(t *testing.T) {
	ports := newFakePorts()
	ports.cells = newUsecaseCellSet(t, [3]string{"cell-1", "123", "feat"})
	ports.cells.Commanders[0].Progress = domain.InReview
	cell, err := (MarkCellInProgressUseCase{Cells: ports}).Execute(context.Background(), MarkCellInProgressInput{Cell: "group-cell-1"})
	if err != nil {
		t.Fatal(err)
	}
	if cell.Progress != domain.InProgress || ports.cells.Commanders[0].Progress != domain.InProgress || ports.cells.Commanders[0].Version != 2 {
		t.Fatalf("cell = %#v, stored = %#v", cell, ports.cells.Commanders[0])
	}
}

func TestMarkCellMergedとAbandonedは終端にする(t *testing.T) {
	for _, from := range []domain.CellProgress{domain.InProgress, domain.InReview} {
		ports := newFakePorts()
		ports.cells = newUsecaseCellSet(t, [3]string{"cell-1", "123", "feat"}, [3]string{"cell-2", "456", "feat"})
		ports.cells.Commanders[0].Progress, ports.cells.Commanders[1].Progress = from, from
		merged, err := (MarkCellMergedUseCase{Cells: ports}).Execute(context.Background(), MarkCellMergedInput{Cell: "123"})
		if err != nil {
			t.Fatal(err)
		}
		abandoned, err := (MarkCellAbandonedUseCase{Cells: ports}).Execute(context.Background(), MarkCellAbandonedInput{Cell: "456"})
		if err != nil {
			t.Fatal(err)
		}
		if merged.Progress != domain.Merged || ports.cells.Commanders[0].Progress != domain.Merged || abandoned.Progress != domain.Abandoned || ports.cells.Commanders[1].Progress != domain.Abandoned {
			t.Fatalf("from %s: stored = %#v", from, ports.cells.Commanders)
		}
	}
}

func TestMarkCellは終端のCellをエラーにして保存しない(t *testing.T) {
	marks := map[string]func(*fakePorts, string) error{
		"review": func(ports *fakePorts, cell string) error {
			_, err := (MarkCellInReviewUseCase{Cells: ports}).Execute(context.Background(), MarkCellInReviewInput{Cell: cell})
			return err
		},
		"implement": func(ports *fakePorts, cell string) error {
			_, err := (MarkCellInProgressUseCase{Cells: ports}).Execute(context.Background(), MarkCellInProgressInput{Cell: cell})
			return err
		},
		"merged": func(ports *fakePorts, cell string) error {
			_, err := (MarkCellMergedUseCase{Cells: ports}).Execute(context.Background(), MarkCellMergedInput{Cell: cell})
			return err
		},
		"abandon": func(ports *fakePorts, cell string) error {
			_, err := (MarkCellAbandonedUseCase{Cells: ports}).Execute(context.Background(), MarkCellAbandonedInput{Cell: cell})
			return err
		},
	}
	for _, terminal := range []domain.CellProgress{domain.Merged, domain.Abandoned} {
		for name, mark := range marks {
			ports := newFakePorts()
			ports.cells = newUsecaseCellSet(t, [3]string{"cell-1", "123", "feat"})
			ports.cells.Commanders[0].Progress = terminal
			if err := mark(ports, "123"); err == nil || !strings.Contains(err.Error(), "cannot move") {
				t.Fatalf("%s on %s: error = %v", name, terminal, err)
			}
			if ports.cells.Commanders[0].Progress != terminal || ports.cells.Commanders[0].Version != 1 {
				t.Fatalf("%s on %s: stored = %#v", name, terminal, ports.cells.Commanders[0])
			}
		}
	}
}

func TestMarkCellは見つからないCellを伝える(t *testing.T) {
	ports := newFakePorts()
	ports.cells = newUsecaseCellSet(t, [3]string{"cell-1", "123", "feat"})
	_, mergedErr := (MarkCellMergedUseCase{Cells: ports}).Execute(context.Background(), MarkCellMergedInput{Cell: "missing"})
	_, abandonErr := (MarkCellAbandonedUseCase{Cells: ports}).Execute(context.Background(), MarkCellAbandonedInput{Cell: "cell-1"})
	for _, err := range []error{mergedErr, abandonErr} {
		if err == nil || !strings.Contains(err.Error(), "not found") {
			t.Fatalf("error = %v", err)
		}
	}
}
