package domain

import "context"

type CellStatePort interface {
	LoadCells(context.Context) ([]Cell, error)
	UpdateCells(context.Context, func([]Cell) ([]Cell, error)) error
	DeleteCell(context.Context, Cell) error
}
