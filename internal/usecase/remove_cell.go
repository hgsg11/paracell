package usecase

import (
	"context"
	"fmt"

	"github.com/shige1114/paradev/internal/domain"
)

type RemoveCellInput struct {
	Cell string
}

type RemoveCellUseCase struct {
	State      CellStatePort
	Source     SourcePort
	Containers ContainerPort
	Session    SessionPort
}

func (u RemoveCellUseCase) Execute(ctx context.Context, input RemoveCellInput) error {
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
	if err := u.Session.RemoveSession(ctx, target); err != nil {
		return err
	}
	if err := u.Containers.RemoveContainers(ctx, target); err != nil {
		return err
	}
	if err := u.Source.RemoveSource(ctx, target); err != nil {
		return err
	}
	next := append([]domain.Cell{}, cells[:index]...)
	next = append(next, cells[index+1:]...)
	return u.State.SaveCells(ctx, next)
}
