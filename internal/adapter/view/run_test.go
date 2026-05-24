package view

import (
	"context"
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/shige1114/paradev/internal/domain"
)

type fakeProgram struct {
	model tea.Model
}

func (p fakeProgram) Run() (tea.Model, error) {
	model := p.model.(Model)
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		updated, _ = updated.(Model).Update(msg)
	}
	if updated.(Model).Error != "" {
		updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	}
	return updated, nil
}

func TestRunはEnter成功で結果を返す(t *testing.T) {
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
	result, err := Run(context.Background(), cells, func(cell domain.Cell) error { return nil }, func(cell domain.Cell) error { return nil }, func(cell domain.Cell) (domain.Cell, error) { return cell, nil })
	if err != nil {
		t.Fatalf("Runでエラーが返った: %v", err)
	}
	if len(got.Cells) != 1 || got.Cells[0].Name != "123" {
		t.Fatalf("cells = %#v, want %#v", got.Cells, cells)
	}
	if result.Action != ActionEnter {
		t.Fatalf("action = %q, want %q", result.Action, ActionEnter)
	}
	if result.Cell.Name != "123" {
		t.Fatalf("cell = %#v, want name %q", result.Cell, "123")
	}
}

func TestRunはEnter失敗後もエラーを表示して継続できる(t *testing.T) {
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
	result, err := Run(context.Background(), cells, func(cell domain.Cell) error {
		return fmt.Errorf("attach failed")
	}, func(cell domain.Cell) error { return nil }, func(cell domain.Cell) (domain.Cell, error) { return cell, nil })
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

func TestRunはlでDone結果を返す(t *testing.T) {
	original := newProgram
	defer func() { newProgram = original }()

	var got Model
	newProgram = func(model tea.Model, opts ...tea.ProgramOption) program {
		_ = opts
		got = model.(Model)
		return programFunc(func() (tea.Model, error) {
			updated, cmd := model.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
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
		func(cell domain.Cell) error { return nil },
		func(cell domain.Cell) error { return nil },
		func(cell domain.Cell) (domain.Cell, error) {
			if err := cell.MarkDone(); err != nil {
				return domain.Cell{}, err
			}
			return cell, nil
		},
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

type programFunc func() (tea.Model, error)

func (f programFunc) Run() (tea.Model, error) {
	return f()
}
