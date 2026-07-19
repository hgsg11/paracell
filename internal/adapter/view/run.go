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

func Run(ctx context.Context, cells []domain.Cell, templates []string, currentCell string, reload func() ([]domain.Cell, error), enter func(domain.Cell) tea.Cmd, goRoot func() error, delete func(domain.Cell) error, markDone func(domain.Cell) (domain.Cell, error), fork func(issue string, template string) tea.Cmd) (Result, error) {
	_ = ctx
	model := NewModel(cells, templates)
	model.CurrentCell = currentCell
	model.Reload = reload
	model.Enter = enter
	model.Delete = delete
	model.MarkDone = markDone
	model.Fork = fork
	p := newProgram(model)
	final, err := p.Run()
	if err != nil {
		return Result{}, err
	}
	model, ok := final.(Model)
	if !ok {
		return Result{}, fmt.Errorf("unexpected view model type %T", final)
	}
	if model.Result.Action == ActionGoRoot && goRoot != nil {
		if err := goRoot(); err != nil {
			return Result{}, err
		}
	}
	return model.Result, nil
}
