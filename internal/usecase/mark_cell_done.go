package usecase

import (
	"context"
	"fmt"

	"github.com/hgsg11/paracell/internal/domain"
)

type MarkCellDoneInput struct {
	Cell string
}

type MarkCellDoneUseCase struct {
	State CellStatePort
}

func (u MarkCellDoneUseCase) Execute(ctx context.Context, input MarkCellDoneInput) (domain.Cell, error) {
	cells, err := u.State.LoadCells(ctx)
	if err != nil {
		return domain.Cell{}, err
	}
	for i, cell := range cells {
		if cell.ID == input.Cell || cell.Issue == input.Cell || cell.Name == input.Cell {
			cell.ToggleDone()
			cells[i] = cell
			if err := u.State.SaveCells(ctx, cells); err != nil {
				return domain.Cell{}, err
			}
			return cell, nil
		}
	}
	return domain.Cell{}, fmt.Errorf("cell %q not found", input.Cell)
}
