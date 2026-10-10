package view

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hgsg11/paracell/internal/adapter/logging"
	"github.com/hgsg11/paracell/internal/domain"
	"github.com/hgsg11/paracell/internal/usecase"
)

type loggerContextKey struct{}

func WithLogger(ctx context.Context, logger *logging.Logger) context.Context {
	return context.WithValue(ctx, loggerContextKey{}, logger)
}

type program interface {
	Run() (tea.Model, error)
}

var newProgram = func(model tea.Model, opts ...tea.ProgramOption) program {
	return tea.NewProgram(model, opts...)
}

func Run(ctx context.Context, cells usecase.CellSet, templates []string, currentCell string, reload func() (usecase.CellSet, error), enter func(domain.CommanderCell) tea.Cmd, goRoot func() error, delete func(domain.CommanderCell) error, markDone func(domain.CommanderCell) (domain.CommanderCell, error), fork func(issue string, template string) tea.Cmd) (Result, error) {
	model := NewModelFromCellSet(cells, templates)
	model.Logger, _ = ctx.Value(loggerContextKey{}).(*logging.Logger)
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
