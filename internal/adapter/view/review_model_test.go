package view

import (
	"strings"
	"testing"

	"github.com/hgsg11/paracell/internal/query"
)

func TestModelはレビュー中のPR番号と判断が要るCellの印を出す(t *testing.T) {
	inReview := viewTestCell("cell-1", "123", "default")
	inReview.Progress, inReview.PullRequests = "in_review", []string{"180"}
	undecided := viewTestCell("cell-2", "456", "default")
	undecided.Progress, undecided.PullRequests, undecided.NeedsDecision = "in_review", []string{"181", "182"}, true
	model := NewModel([]query.CellView{inReview, undecided}, []string{"default"})

	view := stripANSI(model.View())
	if !strings.Contains(view, " #180") || !strings.Contains(view, " !#181,#182") {
		t.Fatalf("view = %q", view)
	}
	if strings.Contains(view, "choose merged or abandon") {
		t.Fatalf("hint shown for a decided cell: %q", view)
	}
	model.Selected = 1
	if view := stripANSI(model.View()); !strings.Contains(view, "! 456: PRs are partly merged and partly closed; choose merged or abandon") {
		t.Fatalf("view = %q", view)
	}
}
