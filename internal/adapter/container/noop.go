package container

import (
	"context"

	"github.com/hgsg11/paracell/internal/domain"
)

type NoopAdapter struct{}

func (a NoopAdapter) CreateContainers(ctx context.Context, cell domain.Cell, templates []domain.ContainerTemplate) error {
	_ = ctx
	_ = cell
	_ = templates
	return nil
}

func (a NoopAdapter) CleanContainers(ctx context.Context, cell domain.Cell) error {
	_ = ctx
	_ = cell
	return nil
}
