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
	var updated domain.Cell
	err := u.State.UpdateCells(ctx, func(cells []domain.Cell) ([]domain.Cell, error) {
		for i, cell := range cells {
			if cell.ID == input.Cell || cell.Issue == input.Cell || cell.Name == input.Cell {
				cell.ToggleDone()
				cells[i] = cell
				updated = cell
				return cells, nil
			}
		}
		return nil, fmt.Errorf("cell %q not found", input.Cell)
	})
	if err != nil {
		return domain.Cell{}, err
	}
	return updated, nil
}
