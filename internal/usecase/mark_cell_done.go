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

func (u MarkCellDoneUseCase) Execute(ctx context.Context, input MarkCellDoneInput) (domain.CommanderCell, error) {
	var updated domain.CommanderCell
	err := u.Cells.UpdateCells(ctx, func(cells CellSet) (CellSet, error) {
		for i, cell := range cells.Commanders {
			if cell.Matches(input.Cell) {
				cell.ToggleDone()
				cells.Commanders[i] = cell
				updated = cell
				return cells, nil
			}
		}
		return CellSet{}, fmt.Errorf("cell %q not found", input.Cell)
	})
	if err != nil {
		return domain.CommanderCell{}, err
	}
	if err := updated.AdvanceVersion(); err != nil {
		return domain.CommanderCell{}, err
	}
	return updated, nil
}
