package usecase

import (
	"context"
	"fmt"

	"github.com/hgsg11/paracell/internal/domain"
)

type CleanCellInput struct {
	Cell string
}

type CleanCellUseCase struct {
	Config           ConfigPort
	State            CellStatePort
	SourceFactory    SourceProviderFactory
	ContainerFactory ContainerProviderFactory
	SessionFactory   SessionProviderFactory
}

func (u CleanCellUseCase) Execute(ctx context.Context, input CleanCellInput) error {
	cfg, err := u.Config.Load(ctx, nil)
	if err != nil {
		return err
	}
	cells, err := u.State.LoadCells(ctx)
	if err != nil {
		return err
	}
	index := -1
	var target domain.Cell
	for i, cell := range cells {
		if cell.ID == input.Cell || cell.Issue == input.Cell || cell.Name == input.Cell {
			index = i
			target = cell
			break
		}
	}
	if index < 0 {
		return fmt.Errorf("cell %q not found", input.Cell)
	}
	if err := target.Clean(); err != nil {
		return err
	}
	session, err := u.SessionFactory.Session(cfg.Providers)
	if err != nil {
		return err
	}
	containers, err := u.ContainerFactory.Container(cfg.Providers)
	if err != nil {
		return err
	}
	source, err := u.SourceFactory.Source(cfg.Providers)
	if err != nil {
		return err
	}
	if err := session.CleanSession(ctx, target); err != nil {
		return err
	}
	if err := containers.CleanContainers(ctx, target); err != nil {
		return err
	}
	if err := source.CleanSource(ctx, target); err != nil {
		return err
	}
	next := append([]domain.Cell{}, cells[:index]...)
	next = append(next, cells[index+1:]...)
	return u.State.SaveCells(ctx, next)
}
