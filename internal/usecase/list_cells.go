package usecase

import (
	"context"

	"github.com/shige1114/paradev/internal/domain"
)

type ListCellsUseCase struct {
	State CellStatePort
}

func (u ListCellsUseCase) Execute(ctx context.Context) ([]domain.Cell, error) {
	return u.State.LoadCells(ctx)
}
