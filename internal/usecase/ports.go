package usecase

import (
	"context"

	"github.com/shige1114/paradev/internal/domain"
)

type ConfigPort interface {
	Load(ctx context.Context) (domain.Config, error)
}

type InitConfigPort interface {
	ConfigExists(ctx context.Context) (bool, error)
	SaveConfig(ctx context.Context, cfg InitConfig) error
}

type CellStatePort interface {
	LoadCells(ctx context.Context) ([]domain.Cell, error)
	SaveCells(ctx context.Context, cells []domain.Cell) error
}

type SourcePort interface {
	CreateSource(ctx context.Context, cell domain.Cell) error
	RemoveSource(ctx context.Context, cell domain.Cell) error
}

type ContainerPort interface {
	CreateContainers(ctx context.Context, cell domain.Cell, template domain.Template) error
	RemoveContainers(ctx context.Context, cell domain.Cell) error
}

type SessionPort interface {
	CreateSession(ctx context.Context, cell domain.Cell) error
	RemoveSession(ctx context.Context, cell domain.Cell) error
}

type IDGenerator interface {
	NewID() string
}
