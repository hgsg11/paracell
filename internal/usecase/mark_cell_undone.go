package usecase

import (
	"context"

	"github.com/hgsg11/paracell/internal/domain"
)

type MarkCellUndoneInput struct {
	Cell string
}

type MarkCellUndoneUseCase struct {
	Cells CellPort
}

// Execute marks the resolved Cell not done. It fails when the Cell is not done.
func (u MarkCellUndoneUseCase) Execute(ctx context.Context, input MarkCellUndoneInput) (domain.CommanderCell, error) {
	commander, _, err := updateCommander(ctx, u.Cells, input.Cell, (*domain.CommanderCell).MarkUndone)
	return commander, err
}
