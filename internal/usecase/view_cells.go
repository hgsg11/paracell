package usecase

import (
	"context"

	"github.com/hgsg11/paracell/internal/domain"
)

type ViewCellsUseCase struct {
	Cells CellPort
}

func (u ViewCellsUseCase) Execute(ctx context.Context) ([]domain.Cell, error) {
	return u.Cells.LoadCells(ctx)
}
