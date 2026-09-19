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
	Cells CellPort
}

func (u MarkCellDoneUseCase) Execute(ctx context.Context, input MarkCellDoneInput) (domain.Cell, error) {
	var updated domain.Cell
	err := u.Cells.UpdateCells(ctx, func(cells []domain.Cell) ([]domain.Cell, error) {
		for i, cell := range cells {
			if cell.Matches(input.Cell) {
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
	if err := updated.AdvanceVersion(); err != nil {
		return domain.Cell{}, err
	}
	return updated, nil
}
