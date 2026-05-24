package usecase

import (
	"context"
	"fmt"

	"github.com/hgsg11/paracell/internal/domain"
)

type RemoveCellInput struct {
	Cell string
}

type RemoveCellUseCase struct {
	Config           ConfigPort
	State            CellStatePort
	SourceFactory    SourceProviderFactory
	ContainerFactory ContainerProviderFactory
	SessionFactory   SessionProviderFactory
}

func (u RemoveCellUseCase) Execute(ctx context.Context, input RemoveCellInput) error {
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
	if !target.CanDelete() {
		return fmt.Errorf("cell %q cannot be deleted until it is done", input.Cell)
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
	if err := session.RemoveSession(ctx, target); err != nil {
		return err
	}
	if err := containers.RemoveContainers(ctx, target); err != nil {
		return err
	}
	if err := source.RemoveSource(ctx, target); err != nil {
		return err
	}
	next := append([]domain.Cell{}, cells[:index]...)
	next = append(next, cells[index+1:]...)
	return u.State.SaveCells(ctx, next)
}
