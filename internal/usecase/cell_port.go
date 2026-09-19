package usecase

import (
	"context"

	"github.com/hgsg11/paracell/internal/domain"
)

type CellPort interface {
	LoadCells(context.Context) ([]domain.Cell, error)
	UpdateCells(context.Context, func([]domain.Cell) ([]domain.Cell, error)) error
	DeleteCell(context.Context, domain.Cell) error
}
