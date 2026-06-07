package view

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hgsg11/paracell/internal/domain"
)

func TestModelはjで選択を下げる(t *testing.T) {
	model := NewModel([]domain.Cell{
		{ID: "cell-1", Name: "123", Template: "default"},
		{ID: "cell-2", Name: "456", Template: "webapp"},
	})

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	got := next.(Model)
	if got.Selected != 1 {
		t.Fatalf("selected = %d, want %d", got.Selected, 1)
	}
}

func TestModelはkで選択を上げる(t *testing.T) {
	model := NewModel([]domain.Cell{
		{ID: "cell-1", Name: "123", Template: "default"},
		{ID: "cell-2", Name: "456", Template: "webapp"},
	})
	model.Selected = 1

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	got := next.(Model)
	if got.Selected != 0 {
		t.Fatalf("selected = %d, want %d", got.Selected, 0)
	}
}

func TestModelは境界で選択を超えない(t *testing.T) {
	model := NewModel([]domain.Cell{
		{ID: "cell-1", Name: "123", Template: "default"},
	})

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	next, _ = next.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	got := next.(Model)
	if got.Selected != 1 {
		t.Fatalf("selected = %d, want %d", got.Selected, 1)
	}
}

func TestModelViewは右端にMarkdownDone列を表示する(t *testing.T) {
	doneCell := domain.Cell{ID: "cell-2", Name: "45678", Template: "web"}
	if err := doneCell.MarkDone(); err != nil {
		t.Fatalf("MarkDoneでエラーが返った: %v", err)
	}
	if err := doneCell.SetStatus(domain.CellStatusReady); err != nil {
		t.Fatalf("SetStatusでエラーが返った: %v", err)
	}
	model := NewModel([]domain.Cell{
		{ID: "cell-1", Name: "123", Template: "default"},
		doneCell,
	})

	got := model.View()
	want := "  NAME   TEMPLATE  STATUS   DONE\n> 123    default   pending  [ ]\n  45678  web       ready    [x]\n\n  exit paracell\n"
	if got != want {
		t.Fatalf("view = %q, want %q", got, want)
	}
}

func TestModelViewは最下部にExitParacellを表示する(t *testing.T) {
	model := NewModel([]domain.Cell{
		{ID: "cell-1", Name: "123", Template: "default"},
	})

	got := model.View()
	want := "  NAME  TEMPLATE  STATUS   DONE\n> 123   default   pending  [ ]\n\n  exit paracell\n"
	if got != want {
		t.Fatalf("view = %q, want %q", got, want)
	}
}

func TestModelはqで終了する(t *testing.T) {
	model := NewModel([]domain.Cell{
		{ID: "cell-1", Name: "123", Template: "default"},
	})

	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	got := next.(Model)
	if !got.Quitting {
		t.Fatal("Quitting = false, want true")
	}
	if cmd == nil {
		t.Fatal("qで終了コマンドが返らなかった")
	}
	if got.Result.Action != ActionQuit {
		t.Fatalf("action = %q, want %q", got.Result.Action, ActionQuit)
	}
}

func TestModelはlで選択中Cellを返す(t *testing.T) {
	model := NewModel([]domain.Cell{
		{ID: "cell-1", Name: "123", Template: "default"},
		{ID: "cell-2", Name: "456", Template: "webapp"},
	})
	model.Selected = 1
	model.Enter = func(cell domain.Cell) tea.Cmd {
		return func() tea.Msg { return enterResultMsg{cell: cell, err: nil} }
	}

	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if cmd == nil {
		t.Fatal("lでコマンドが返らなかった")
	}
	updated, nextCmd := next.(Model).Update(cmd())
	got := updated.(Model)
	if got.Result.Action != ActionEnter {
		t.Fatalf("action = %q, want %q", got.Result.Action, ActionEnter)
	}
	if got.Result.Cell.Name != "456" {
		t.Fatalf("cell = %#v, want name %q", got.Result.Cell, "456")
	}
	if nextCmd == nil {
		t.Fatal("l成功で終了コマンドが返らなかった")
	}
}

func TestModelはEnterで選択中CellのDoneを切り替える(t *testing.T) {
	model := NewModel([]domain.Cell{
		{ID: "cell-1", Name: "123", Template: "default"},
	})
	model.MarkDone = func(cell domain.Cell) (domain.Cell, error) {
		if cell.Name != "123" {
			t.Fatalf("mark done cell = %#v, want name %q", cell, "123")
		}
		if err := cell.MarkDone(); err != nil {
			t.Fatalf("MarkDoneでエラーが返った: %v", err)
		}
		return cell, nil
	}
	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enterでコマンドが返らなかった")
	}
	updated, nextCmd := next.(Model).Update(cmd())
	got := updated.(Model)
	if !got.Cells[0].IsDone() {
		t.Fatal("IsDone = false, want true")
	}
	if got.Result.Action != ActionNone {
		t.Fatalf("action = %q, want %q", got.Result.Action, ActionNone)
	}
	if nextCmd != nil {
		t.Fatal("Enterで終了コマンドが返った")
	}
}

func TestModelはddで選択中Cellを削除する(t *testing.T) {
	model := NewModel([]domain.Cell{
		{ID: "cell-1", Name: "123", Template: "default"},
		{ID: "cell-2", Name: "456", Template: "webapp"},
	})
	model.Delete = func(cell domain.Cell) error {
		if cell.Name != "123" {
			t.Fatalf("delete cell = %#v, want name %q", cell, "123")
		}
		return nil
	}

	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	got := next.(Model)
	if !got.AwaitingDelete {
		t.Fatal("AwaitingDelete = false, want true")
	}
	if cmd != nil {
		t.Fatal("最初のdでコマンドが返った")
	}

	next, cmd = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if cmd == nil {
		t.Fatal("ddでコマンドが返らなかった")
	}
	updated, nextCmd := next.(Model).Update(cmd())
	got = updated.(Model)
	if got.Error != "" {
		t.Fatalf("error = %q, want empty", got.Error)
	}
	if len(got.Cells) != 1 || got.Cells[0].Name != "456" {
		t.Fatalf("cells = %#v, want remaining cell 456", got.Cells)
	}
	if got.Result.Action != ActionDelete {
		t.Fatalf("action = %q, want %q", got.Result.Action, ActionDelete)
	}
	if nextCmd != nil {
		t.Fatal("delete成功で終了コマンドが返った")
	}
}

func TestModelはExitParacellをCleanできない(t *testing.T) {
	model := NewModel([]domain.Cell{
		{ID: "cell-1", Name: "123", Template: "default"},
	})
	model.Selected = 1
	model.Delete = func(cell domain.Cell) error {
		t.Fatalf("exit paracellでdelete handlerが呼ばれた: %#v", cell)
		return nil
	}

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	next, cmd := next.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})

	if cmd != nil {
		t.Fatal("exit paracellのddでコマンドが返った")
	}
	got := next.(Model)
	if got.Error != "exit paracell cannot be cleaned" {
		t.Fatalf("error = %q, want %q", got.Error, "exit paracell cannot be cleaned")
	}
}

func TestModelはExitParacellをDoneにできない(t *testing.T) {
	model := NewModel([]domain.Cell{
		{ID: "cell-1", Name: "123", Template: "default"},
	})
	model.Selected = 1
	model.MarkDone = func(cell domain.Cell) (domain.Cell, error) {
		t.Fatalf("exit paracellでmark done handlerが呼ばれた: %#v", cell)
		return cell, nil
	}

	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if cmd != nil {
		t.Fatal("exit paracellのEnterでコマンドが返った")
	}
	got := next.(Model)
	if got.Error != "exit paracell cannot be marked done" {
		t.Fatalf("error = %q, want %q", got.Error, "exit paracell cannot be marked done")
	}
}

func TestModelはdのあと別キーなら削除待機を解除する(t *testing.T) {
	model := NewModel([]domain.Cell{
		{ID: "cell-1", Name: "123", Template: "default"},
	})

	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if cmd != nil {
		t.Fatal("最初のdでコマンドが返った")
	}
	got := next.(Model)
	if !got.AwaitingDelete {
		t.Fatal("AwaitingDelete = false, want true")
	}

	next, cmd = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if cmd != nil {
		t.Fatal("dのあとjでコマンドが返った")
	}
	got = next.(Model)
	if got.AwaitingDelete {
		t.Fatal("AwaitingDelete = true, want false")
	}
	if got.Selected != 1 {
		t.Fatalf("selected = %d, want %d", got.Selected, 1)
	}
}

func TestModelはEnterでdone状態のCellを解除する(t *testing.T) {
	model := NewModel([]domain.Cell{
		func() domain.Cell {
			cell := domain.Cell{ID: "cell-1", Name: "123", Template: "default"}
			if err := cell.MarkDone(); err != nil {
				t.Fatalf("MarkDoneでエラーが返った: %v", err)
			}
			return cell
		}(),
	})
	model.MarkDone = func(cell domain.Cell) (domain.Cell, error) {
		if cell.Name != "123" {
			t.Fatalf("toggle cell = %#v, want name %q", cell, "123")
		}
		return domain.Cell{ID: cell.ID, Name: cell.Name, Template: cell.Template}, nil
	}

	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enterでコマンドが返らなかった")
	}
	updated, _ := next.(Model).Update(cmd())
	if updated.(Model).Cells[0].IsDone() {
		t.Fatal("IsDone = true, want false")
	}
}
