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
}

// Execute marks the resolved Cell merged. It fails when the Cell is already merged or abandoned.
func (u MarkCellMergedUseCase) Execute(ctx context.Context, input MarkCellMergedInput) (domain.CommanderCell, error) {
	commander, _, err := updateCommander(ctx, u.Cells, input.Cell, (*domain.CommanderCell).MarkMerged)
	return commander, err
}
