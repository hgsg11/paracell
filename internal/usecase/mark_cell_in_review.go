package usecase

import (
	"context"

	"github.com/hgsg11/paracell/internal/domain"
)

type MarkCellInReviewInput struct {
	Cell string
}

type MarkCellInReviewUseCase struct {
	Cells CellPort
}

// Execute marks the resolved Cell in review. It fails unless the Cell is in progress.
func (u MarkCellInReviewUseCase) Execute(ctx context.Context, input MarkCellInReviewInput) (domain.CommanderCell, error) {
	commander, _, err := updateCommander(ctx, u.Cells, input.Cell, (*domain.CommanderCell).MarkInReview)
	return commander, err
}
