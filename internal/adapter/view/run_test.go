package view

import (
	"context"
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hgsg11/paracell/internal/domain"
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

func TestRunはspace成功で結果を返す(t *testing.T) {
	original := newProgram
	defer func() { newProgram = original }()

	var got Model
	newProgram = func(model tea.Model, opts ...tea.ProgramOption) program {
		_ = opts
		got = model.(Model)
		return fakeProgram{model: model}
	}

	cells := []domain.Cell{
		{ID: "cell-1", Name: "123", Template: "default"},
	}
	result, err := Run(context.Background(), cells, nil, "123", func() ([]domain.Cell, error) {
		return cells, nil
	}, func(cell domain.Cell) tea.Cmd {
		if cell.Name != "123" {
			t.Fatalf("enter cell = %#v, want name %q", cell, "123")
		}
		return func() tea.Msg { return enterResultMsg{cell: cell, err: nil} }
	}, func() error { return nil }, func(cell domain.Cell) error { return nil }, func(cell domain.Cell) (domain.Cell, error) { return cell, nil }, nil)
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
	if result.Cell.Name != "123" {
		t.Fatalf("cell = %#v, want name %q", result.Cell, "123")
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

	cells := []domain.Cell{
		{ID: "cell-1", Name: "123", Template: "default"},
	}
	result, err := Run(context.Background(), cells, nil, "", func() ([]domain.Cell, error) {
		return cells, nil
	}, func(cell domain.Cell) tea.Cmd {
		return func() tea.Msg { return enterResultMsg{cell: cell, err: fmt.Errorf("attach failed")} }
	}, func() error { return nil }, func(cell domain.Cell) error { return nil }, func(cell domain.Cell) (domain.Cell, error) { return cell, nil }, nil)
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

	var got Model
	newProgram = func(model tea.Model, opts ...tea.ProgramOption) program {
		_ = opts
		got = model.(Model)
		return programFunc(func() (tea.Model, error) {
			updated, cmd := model.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
			if cmd != nil {
				updated, _ = updated.(Model).Update(cmd())
			}
			return updated, nil
		})
	}

	cells := []domain.Cell{
		{ID: "cell-1", Name: "123", Template: "default"},
	}
	result, err := Run(
		context.Background(),
		cells,
		nil,
		"",
		func() ([]domain.Cell, error) { return cells, nil },
		func(cell domain.Cell) tea.Cmd { return nil },
		func() error { return nil },
		func(cell domain.Cell) error { return nil },
		func(cell domain.Cell) (domain.Cell, error) {
			if err := cell.MarkDone(); err != nil {
				return domain.Cell{}, err
			}
			return cell, nil
		},
		nil,
	)
	if err != nil {
		t.Fatalf("Runでエラーが返った: %v", err)
	}
	if result.Action != ActionNone {
		t.Fatalf("action = %q, want %q", result.Action, ActionNone)
	}
	if !got.Cells[0].IsDone() {
		t.Fatal("IsDone = false, want true")
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
		[]domain.Cell{{ID: "cell-1", Name: "123", Template: "default"}},
		nil,
		"",
		func() ([]domain.Cell, error) {
			return []domain.Cell{{ID: "cell-1", Name: "123", Template: "default"}}, nil
		},
		func(cell domain.Cell) tea.Cmd { return nil },
		func() error {
			goRootCalled = true
			return nil
		},
		func(cell domain.Cell) error { return nil },
		func(cell domain.Cell) (domain.Cell, error) { return cell, nil },
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
	newProgram = func(model tea.Model, opts ...tea.ProgramOption) program {
		_ = opts
		return programFunc(func() (tea.Model, error) {
			m := model.(Model)
			m.Focus = FocusTemplates
			m.IssueInputActive = true
			m.ForkTemplate = "default"
			m.IssueInput = "123"
			updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			if cmd != nil {
				t.Fatal("issue入力のEnterでforkコマンドが返った")
			}
			updated, cmd = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
			if cmd == nil {
				t.Fatal("input入力のEnterでforkコマンドが返らなかった")
			}
			updated, _ = updated.(Model).Update(cmd())
			return updated, nil
		})
	}

	_, err := Run(
		context.Background(),
		nil,
		[]string{"default"},
		"",
		func() ([]domain.Cell, error) {
			reloaded = true
			return []domain.Cell{{ID: "cell-1", Name: "123", Template: "default"}}, nil
		},
		func(cell domain.Cell) tea.Cmd { return nil },
		func() error { return nil },
		func(cell domain.Cell) error { return nil },
		func(cell domain.Cell) (domain.Cell, error) { return cell, nil },
		func(issue string, template string, input string) tea.Cmd {
			return func() tea.Msg {
				return forkResultMsg{cell: domain.Cell{ID: "cell-1", Name: "123", Template: "default"}}
			}
		},
	)
	if err != nil {
		t.Fatalf("Runでエラーが返った: %v", err)
	}
	if !reloaded {
		t.Fatal("reloadが呼ばれなかった")
	}
}

type programFunc func() (tea.Model, error)

func (f programFunc) Run() (tea.Model, error) {
	return f()
}
