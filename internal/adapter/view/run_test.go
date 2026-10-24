package view

import (
	"context"
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hgsg11/paracell/internal/query"
)

type fakeProgram struct {
	model tea.Model
}

func (p fakeProgram) Run() (tea.Model, error) {
	model := p.model.(Model)
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeySpace})
	if cmd != nil {
		msg := cmd()
		updated, _ = updated.(Model).Update(msg)
	}
	if updated.(Model).Error != "" {
		updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	}
	return updated, nil
}

func noCellAction(string) error { return nil }

func TestRunはspace成功で結果を返す(t *testing.T) {
	original := newProgram
	defer func() { newProgram = original }()

	var got Model
	newProgram = func(model tea.Model, opts ...tea.ProgramOption) program {
		_ = opts
		got = model.(Model)
		return fakeProgram{model: model}
	}

	cells := []query.CellView{
		viewTestCell("cell-1", "123", "default"),
	}
	result, err := Run(context.Background(), cells, nil, "123", func() ([]query.CellView, error) {
		return cells, nil
	}, func(name string) tea.Cmd {
		if name != "123" {
			t.Fatalf("enter cell = %q, want %q", name, "123")
		}
		return func() tea.Msg { return enterResultMsg{name: name, err: nil} }
	}, func() error { return nil }, noCellAction, noCellAction, noCellAction, nil)
	if err != nil {
		t.Fatalf("Runでエラーが返った: %v", err)
	}
	if len(got.Cells) != 1 || got.Cells[0].Name != "123" {
		t.Fatalf("cells = %#v, want %#v", got.Cells, cells)
	}
	if got.CurrentCell != "123" {
		t.Fatalf("current cell = %q, want %q", got.CurrentCell, "123")
	}
	if result.Action != ActionEnter {
		t.Fatalf("action = %q, want %q", result.Action, ActionEnter)
	}
	if result.Cell != "123" {
		t.Fatalf("cell = %q, want %q", result.Cell, "123")
	}
}

func TestRunはspace失敗後もエラーを表示して継続できる(t *testing.T) {
	original := newProgram
	defer func() { newProgram = original }()

	var observed Model
	newProgram = func(model tea.Model, opts ...tea.ProgramOption) program {
		_ = opts
		return programFunc(func() (tea.Model, error) {
			p := fakeProgram{model: model}
			updated, err := p.Run()
			observed = updated.(Model)
			return updated, err
		})
	}

	cells := []query.CellView{
		viewTestCell("cell-1", "123", "default"),
	}
	result, err := Run(context.Background(), cells, nil, "", func() ([]query.CellView, error) {
		return cells, nil
	}, func(name string) tea.Cmd {
		return func() tea.Msg { return enterResultMsg{name: name, err: fmt.Errorf("attach failed")} }
	}, func() error { return nil }, noCellAction, noCellAction, noCellAction, nil)
	if err != nil {
		t.Fatalf("Runでエラーが返った: %v", err)
	}
	if result.Action != ActionQuit {
		t.Fatalf("action = %q, want %q", result.Action, ActionQuit)
	}
	if observed.Error != "attach failed" {
		t.Fatalf("error = %q, want %q", observed.Error, "attach failed")
	}
}

func TestRunはEnterでDone状態を切り替える(t *testing.T) {
	original := newProgram
	defer func() { newProgram = original }()

	var final Model
	newProgram = func(model tea.Model, opts ...tea.ProgramOption) program {
		_ = opts
		return programFunc(func() (tea.Model, error) {
			updated, cmd := model.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
			if cmd != nil {
				updated, _ = updated.(Model).Update(cmd())
			}
			final = updated.(Model)
			return updated, nil
		})
	}

	cells := []query.CellView{
		viewTestCell("cell-1", "123", "default"),
	}
	var markedDone []string
	result, err := Run(
		context.Background(),
		cells,
		nil,
		"",
		func() ([]query.CellView, error) { return cells, nil },
		func(string) tea.Cmd { return nil },
		func() error { return nil },
		noCellAction,
		func(name string) error {
			markedDone = append(markedDone, name)
			return nil
		},
		func(name string) error {
			t.Fatalf("mark undone was called for %q", name)
			return nil
		},
		nil,
	)
	if err != nil {
		t.Fatalf("Runでエラーが返った: %v", err)
	}
	if result.Action != ActionNone {
		t.Fatalf("action = %q, want %q", result.Action, ActionNone)
	}
	if len(markedDone) != 1 || markedDone[0] != "123" || !final.Cells[0].Done {
		t.Fatalf("mark done calls = %#v, cells = %#v", markedDone, final.Cells)
	}
}

func TestRunはGoRoot選択後にGoRoot処理を実行する(t *testing.T) {
	original := newProgram
	defer func() { newProgram = original }()

	goRootCalled := false
	newProgram = func(model tea.Model, opts ...tea.ProgramOption) program {
		_ = opts
		return programFunc(func() (tea.Model, error) {
			updated, cmd := model.(Model).Update(tea.KeyMsg{Type: tea.KeyTab})
			_ = cmd
			updated, cmd = updated.(Model).Update(tea.KeyMsg{Type: tea.KeySpace})
			if cmd == nil {
				t.Fatal("exit選択で終了コマンドが返らなかった")
			}
			return updated, nil
		})
	}

	result, err := Run(
		context.Background(),
		[]query.CellView{viewTestCell("cell-1", "123", "default")},
		nil,
		"",
		func() ([]query.CellView, error) {
			return []query.CellView{viewTestCell("cell-1", "123", "default")}, nil
		},
		func(string) tea.Cmd { return nil },
		func() error {
			goRootCalled = true
			return nil
		},
		noCellAction,
		noCellAction,
		noCellAction,
		nil,
	)
	if err != nil {
		t.Fatalf("Runでエラーが返った: %v", err)
	}
	if !goRootCalled {
		t.Fatal("go root handlerが呼ばれなかった")
	}
	if result.Action != ActionGoRoot {
		t.Fatalf("action = %q, want %q", result.Action, ActionGoRoot)
	}
}

func TestRunはFork成功後にReloadされたCellを保持する(t *testing.T) {
	original := newProgram
	defer func() { newProgram = original }()

	reloaded := false
	var final Model
	newProgram = func(model tea.Model, opts ...tea.ProgramOption) program {
		_ = opts
		return programFunc(func() (tea.Model, error) {
			m := model.(Model)
			m.Focus = FocusTemplates
			m.IssueInputActive = true
			m.ForkTemplate = "default"
			m.IssueInput = "123"
			updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			if cmd == nil {
				t.Fatal("issue入力のEnterでforkコマンドが返らなかった")
			}
			updated, _ = updated.(Model).Update(cmd())
			final = updated.(Model)
			return updated, nil
		})
	}

	_, err := Run(
		context.Background(),
		nil,
		[]string{"default"},
		"",
		func() ([]query.CellView, error) {
			reloaded = true
			return []query.CellView{viewTestCell("cell-1", "123", "default")}, nil
		},
		func(string) tea.Cmd { return nil },
		func() error { return nil },
		noCellAction,
		noCellAction,
		noCellAction,
		func(issue string, template string) tea.Cmd {
			return ForkResultCmd(nil)
		},
	)
	if err != nil {
		t.Fatalf("Runでエラーが返った: %v", err)
	}
	if !reloaded || len(final.Cells) != 1 || final.Cells[0].Name != "123" {
		t.Fatalf("reloaded = %t, cells = %#v", reloaded, final.Cells)
	}
}

type programFunc func() (tea.Model, error)

func (f programFunc) Run() (tea.Model, error) {
	return f()
}
