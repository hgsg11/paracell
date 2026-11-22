package view

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hgsg11/paracell/internal/query"
)

func TestModelは待ちのエントリをCellの下に選べない行として出す(t *testing.T) {
	model := NewModel([]query.CellView{viewTestCell("cell-1", "123", "default")}, []string{"default"})
	model.Reload = func() ([]query.CellView, error) { return model.Cells, nil }
	model.ReloadQueue = func() ([]query.QueueView, error) {
		return []query.QueueView{
			{Issue: "123", State: query.QueueStateRunning},
			{Issue: "456", Note: "API", State: query.QueueStateWaiting, BlockedBy: []string{"123"}},
		}, nil
	}
	updated, _ := model.Update(refreshMsg{})
	model = updated.(Model)
	if len(model.Waiting) != 1 || model.Waiting[0].Issue != "456" {
		t.Fatalf("waiting = %#v", model.Waiting)
	}
	lines := strings.Split(stripANSI(model.View()), "\n")
	if !strings.Contains(lines[3], "API  wait 123") {
		t.Fatalf("waiting row missing: %q", lines)
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if updated.(Model).Selected != 0 {
		t.Fatalf("selection moved onto a waiting entry: %d", updated.(Model).Selected)
	}
}
