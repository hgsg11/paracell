package view

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hgsg11/paracell/internal/domain"
)

type program interface {
	Run() (tea.Model, error)
}

var newProgram = func(model tea.Model, opts ...tea.ProgramOption) program {
	return tea.NewProgram(model, opts...)
}

func Run(ctx context.Context, cells []domain.Cell, enter func(domain.Cell) error, delete func(domain.Cell) error, markDone func(domain.Cell) (domain.Cell, error)) (Result, error) {
	_ = ctx
	model := NewModel(cells)
	model.Enter = enter
	model.Delete = delete
	model.MarkDone = markDone
	p := newProgram(model)
	final, err := p.Run()
	if err != nil {
		return Result{}, err
	}
	model, ok := final.(Model)
	if !ok {
		return Result{}, fmt.Errorf("unexpected view model type %T", final)
	}
	return model.Result, nil
}
