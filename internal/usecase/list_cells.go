package usecase

import (
	"context"

	"github.com/hgsg11/paracell/internal/domain"
)

type ListCellsUseCase struct {
	Cells CellPort
}

func (u ListCellsUseCase) Execute(ctx context.Context) ([]domain.Cell, error) {
	return u.Cells.LoadCells(ctx)
}
