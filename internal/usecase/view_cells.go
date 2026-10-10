package usecase

import (
	"context"
)

type ViewCellsUseCase struct {
	Cells CellPort
}

func (u ViewCellsUseCase) Execute(ctx context.Context) (CellSet, error) {
	return u.Cells.LoadCells(ctx)
}
