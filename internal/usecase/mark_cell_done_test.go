package usecase

import (
	"context"
	"strings"
	"testing"
)

func TestMarkCellDoneは未完了のCellを完了にする(t *testing.T) {
	ports := newFakePorts()
	ports.cells = newUsecaseCellSet(t, [3]string{"cell-1", "123", "feat"}, [3]string{"cell-2", "456", "feat"})
	cell, err := (MarkCellDoneUseCase{Cells: ports}).Execute(context.Background(), MarkCellDoneInput{Cell: "456"})
	if err != nil {
		t.Fatal(err)
	}
	if !cell.Done || cell.ID != "cell-2" || !ports.cells.Commanders[1].Done || ports.cells.Commanders[1].Version != 2 {
		t.Fatalf("cell = %#v, stored = %#v", cell, ports.cells.Commanders)
	}
	if ports.cells.Commanders[0].Done || ports.cells.Commanders[0].Version != 1 {
		t.Fatalf("another cell changed: %#v", ports.cells.Commanders[0])
	}
}

func TestMarkCellDoneは完了済みのCellをエラーにして保存しない(t *testing.T) {
	ports := newFakePorts()
	ports.cells = newUsecaseCellSet(t, [3]string{"cell-1", "123", "feat"})
	ports.cells.Commanders[0].Done = true
	if _, err := (MarkCellDoneUseCase{Cells: ports}).Execute(context.Background(), MarkCellDoneInput{Cell: "123"}); err == nil || !strings.Contains(err.Error(), "already done") {
		t.Fatalf("error = %v", err)
	}
	if !ports.cells.Commanders[0].Done || ports.cells.Commanders[0].Version != 1 {
		t.Fatalf("stored = %#v", ports.cells.Commanders[0])
	}
}

func TestMarkCellUndoneは完了済みのCellを未完了に戻す(t *testing.T) {
	ports := newFakePorts()
	ports.cells = newUsecaseCellSet(t, [3]string{"cell-1", "123", "feat"})
	ports.cells.Commanders[0].Done = true
	cell, err := (MarkCellUndoneUseCase{Cells: ports}).Execute(context.Background(), MarkCellUndoneInput{Cell: "group-cell-1"})
	if err != nil {
		t.Fatal(err)
	}
	if cell.Done || ports.cells.Commanders[0].Done || ports.cells.Commanders[0].Version != 2 {
		t.Fatalf("cell = %#v, stored = %#v", cell, ports.cells.Commanders[0])
	}
}

func TestMarkCellUndoneは未完了のCellをエラーにして保存しない(t *testing.T) {
	ports := newFakePorts()
	ports.cells = newUsecaseCellSet(t, [3]string{"cell-1", "123", "feat"})
	if _, err := (MarkCellUndoneUseCase{Cells: ports}).Execute(context.Background(), MarkCellUndoneInput{Cell: "123"}); err == nil || !strings.Contains(err.Error(), "not done") {
		t.Fatalf("error = %v", err)
	}
	if ports.cells.Commanders[0].Done || ports.cells.Commanders[0].Version != 1 {
		t.Fatalf("stored = %#v", ports.cells.Commanders[0])
	}
}

func TestMarkCellDoneとUndoneは見つからないCellを伝える(t *testing.T) {
	ports := newFakePorts()
	ports.cells = newUsecaseCellSet(t, [3]string{"cell-1", "123", "feat"})
	_, doneErr := (MarkCellDoneUseCase{Cells: ports}).Execute(context.Background(), MarkCellDoneInput{Cell: "missing"})
	_, undoneErr := (MarkCellUndoneUseCase{Cells: ports}).Execute(context.Background(), MarkCellUndoneInput{Cell: "cell-1"})
	for _, err := range []error{doneErr, undoneErr} {
		if err == nil || !strings.Contains(err.Error(), "not found") {
			t.Fatalf("error = %v", err)
		}
	}
}
