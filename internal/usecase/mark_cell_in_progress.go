package usecase

import (
	"context"

	"github.com/hgsg11/paracell/internal/domain"
)

type MarkCellInProgressInput struct {
	Cell string
}

type MarkCellInProgressUseCase struct {
	Cells CellPort
}

// Execute moves the resolved Cell back to in progress. It fails unless the Cell is in review.
func (u MarkCellInProgressUseCase) Execute(ctx context.Context, input MarkCellInProgressInput) (domain.CommanderCell, error) {
	commander, _, err := updateCommander(ctx, u.Cells, input.Cell, (*domain.CommanderCell).MarkInProgress)
	return commander, err
}
