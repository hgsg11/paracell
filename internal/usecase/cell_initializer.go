package usecase

import "context"

type CellInitializer interface {
	Initialize(context.Context) error
}
