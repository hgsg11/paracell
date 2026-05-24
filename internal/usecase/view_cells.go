package usecase

import (
	"context"

	"github.com/hgsg11/paracell/internal/domain"
)

type ViewCellsUseCase struct {
	State CellStatePort
}

func (u ViewCellsUseCase) Execute(ctx context.Context) ([]domain.Cell, error) {
	return u.State.LoadCells(ctx)
}
