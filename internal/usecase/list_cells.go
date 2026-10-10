package usecase

import (
	"context"
)

type ListCellsUseCase struct {
	Cells CellPort
}

func (u ListCellsUseCase) Execute(ctx context.Context) (CellSet, error) {
	return u.Cells.LoadCells(ctx)
}
