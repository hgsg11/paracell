package container

import (
	"context"

	"github.com/hgsg11/paracell/internal/domain"
)

type NoopAdapter struct{}

func (a NoopAdapter) CreateContainers(ctx context.Context, cell domain.Cell, template domain.Template) error {
	_ = ctx
	_ = cell
	_ = template
	return nil
}

func (a NoopAdapter) CleanContainers(ctx context.Context, cell domain.Cell) error {
	_ = ctx
	_ = cell
	return nil
}
