package usecase

import (
	"context"

	"github.com/hgsg11/paracell/internal/domain"
)

type MarkCellAbandonedInput struct {
	Cell string
}

type MarkCellAbandonedUseCase struct {
	Cells CellPort
}

// Execute marks the resolved Cell abandoned. It fails when the Cell is already merged or abandoned.
func (u MarkCellAbandonedUseCase) Execute(ctx context.Context, input MarkCellAbandonedInput) (domain.CommanderCell, error) {
	commander, _, err := updateCommander(ctx, u.Cells, input.Cell, (*domain.CommanderCell).MarkAbandoned)
	return commander, err
}
