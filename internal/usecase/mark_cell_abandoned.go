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
	// Fork starts the queued Cells released by this one.
	Fork ForkCellUseCase
}

// Execute marks the resolved Cell abandoned. It fails when the Cell is already merged or abandoned.
// The QueueEntry the Cell started from is removed in the same state update, and
// the queued Cells no longer blocked are started afterwards.
func (u MarkCellAbandonedUseCase) Execute(ctx context.Context, input MarkCellAbandonedInput) (domain.CommanderCell, error) {
	return finishCommander(ctx, u.Cells, u.Fork, input.Cell, (*domain.CommanderCell).MarkAbandoned)
}
