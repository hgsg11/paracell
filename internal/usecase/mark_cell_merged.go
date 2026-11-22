package usecase

import (
	"context"

	"github.com/hgsg11/paracell/internal/domain"
)

type MarkCellMergedInput struct {
	Cell string
}

type MarkCellMergedUseCase struct {
	Cells CellPort
	// Fork starts the queued Cells released by this one.
	Fork ForkCellUseCase
}

// Execute marks the resolved Cell merged. It fails when the Cell is already merged or abandoned.
// The QueueEntry the Cell started from is removed in the same state update, and
// the queued Cells no longer blocked are started afterwards.
func (u MarkCellMergedUseCase) Execute(ctx context.Context, input MarkCellMergedInput) (domain.CommanderCell, error) {
	return finishCommander(ctx, u.Cells, u.Fork, input.Cell, (*domain.CommanderCell).MarkMerged)
}
