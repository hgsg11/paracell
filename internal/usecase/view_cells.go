package usecase

import (
	"context"

	"github.com/shige1114/paradev/internal/domain"
)

type ViewCellsUseCase struct {
	State CellStatePort
}

func (u ViewCellsUseCase) Execute(ctx context.Context) ([]domain.Cell, error) {
	return u.State.LoadCells(ctx)
}
