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
		cell, ok := cells.FindCommander(input.Cell)
		if !ok {
			return CellSet{}, fmt.Errorf("cell %q not found", input.Cell)
		}
		cell.ToggleDone()
		for i := range cells.Commanders {
			if cells.Commanders[i].ID == cell.ID {
				cells.Commanders[i] = cell
				break
			}
		}
		updated = cell
		return cells, nil
	})
	if err != nil {
		return domain.CommanderCell{}, err
	}
	if err := updated.AdvanceVersion(); err != nil {
		return domain.CommanderCell{}, err
	}
	return updated, nil
}
