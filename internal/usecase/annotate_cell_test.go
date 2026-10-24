package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/hgsg11/paracell/internal/domain"
)

func newAnnotateTestPorts(t *testing.T) *fakePorts {
	t.Helper()
	ports := newFakePorts()
	ports.cells = newUsecaseCellSet(t, [3]string{"cell-1", "123", "feat"})
	return ports
}

func TestAnnotateCellはCellGroupのIDとIssueとNameでNoteを設定上書きする(t *testing.T) {
	selectors := []string{"group-cell-1", "123"}
	for _, selector := range selectors {
		t.Run(selector, func(t *testing.T) {
			ports := newAnnotateTestPorts(t)
			_ = ports.cells.Groups[0].SetNote("旧案")
			updated, err := (AnnotateCellUseCase{Cells: ports, WorkspaceFactory: ports}).Execute(context.Background(), AnnotateCellInput{Cell: selector, Note: "  API\t実装\n中 "})
			if err != nil {
				t.Fatal(err)
			}
			if updated.Note != "API 実装 中" || ports.cells.Groups[0].Note != "API 実装 中" {
				t.Fatalf("updated note = %q, stored = %q", updated.Note, ports.cells.Groups[0].Note)
			}
			if ports.cells.Commanders[0].Version != 2 {
				t.Fatalf("row version = %d", ports.cells.Commanders[0].Version)
			}
			if got := ports.calls[len(ports.calls)-1]; got != "workspace:label:API 実装 中" {
				t.Fatalf("last call = %q", got)
			}
		})
	}
}

func TestAnnotateCellはWorkspaceなしを成功扱いにする(t *testing.T) {
	ports := newAnnotateTestPorts(t)
	ports.updateStatusLabelErr = domain.ErrNotFound
	updated, err := (AnnotateCellUseCase{Cells: ports, WorkspaceFactory: ports}).Execute(context.Background(), AnnotateCellInput{Cell: "123", Note: "検証中"})
	if err != nil || updated.Note != "検証中" || ports.cells.Groups[0].Note != "検証中" {
		t.Fatalf("updated = %#v, stored = %#v, error = %v", updated, ports.cells, err)
	}
}

func TestAnnotateCellはStatus更新失敗時に保存済みと伝える(t *testing.T) {
	ports := newAnnotateTestPorts(t)
	ports.updateStatusLabelErr = errors.New("tmux unavailable")
	updated, err := (AnnotateCellUseCase{Cells: ports, WorkspaceFactory: ports}).Execute(context.Background(), AnnotateCellInput{Cell: "123", Note: "検証中"})
	if err == nil || !strings.Contains(err.Error(), "cell note was saved") {
		t.Fatalf("error = %v", err)
	}
	if updated.Note != "検証中" || ports.cells.Groups[0].Note != "検証中" {
		t.Fatalf("note was not preserved: updated=%#v stored=%#v", updated, ports.cells)
	}
}

func TestAnnotateCellは不正Noteと存在しないCellを保存しない(t *testing.T) {
	for _, input := range []AnnotateCellInput{
		{Cell: "123", Note: ""},
		{Cell: "123", Note: strings.Repeat("a", 21)},
		{Cell: "missing", Note: "検証中"},
		{Cell: "cell-1", Note: "検証中"},
	} {
		ports := newAnnotateTestPorts(t)
		_, err := (AnnotateCellUseCase{Cells: ports, WorkspaceFactory: ports}).Execute(context.Background(), input)
		if err == nil {
			t.Fatalf("input %#v returned no error", input)
		}
		if len(ports.calls) != 0 || ports.cells.Groups[0].Note != "" || ports.cells.Commanders[0].Version != 1 {
			t.Fatalf("input %#v calls = %#v, stored = %#v", input, ports.calls, ports.cells)
		}
	}
}
