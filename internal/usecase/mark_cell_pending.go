package usecase

import (
	"context"

	"github.com/hgsg11/paracell/internal/domain"
)

type MarkCellPendingInput struct {
	Cell string
}

// MarkCellPendingUseCase backs `paracell pending`. It sends no notification.
type MarkCellPendingUseCase struct {
	Cells CellPort
}

func (u MarkCellPendingUseCase) Execute(ctx context.Context, input MarkCellPendingInput) (domain.CommanderCell, error) {
	commander, _, err := updateCommander(ctx, u.Cells, input.Cell, func(commander *domain.CommanderCell) error {
		commander.MarkPending()
		return nil
	})
	return commander, err
}
