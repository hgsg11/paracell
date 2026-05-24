package usecase

import (
	"context"

	"github.com/hgsg11/paracell/internal/domain"
)

type ListCellsUseCase struct {
	State CellStatePort
}

func (u ListCellsUseCase) Execute(ctx context.Context) ([]domain.Cell, error) {
	return u.State.LoadCells(ctx)
}
