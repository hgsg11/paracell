package container

import (
	"context"

	"github.com/hgsg11/paracell/internal/domain"
)

type NoopAdapter struct{}

func NewNoopAdapter() NoopAdapter { return NoopAdapter{} }

func (a NoopAdapter) CreateContainers(ctx context.Context, resources domain.ContainerResources) (map[string][]string, error) {
	_ = ctx
	_ = resources
	return map[string][]string{}, nil
}

func (a NoopAdapter) CleanContainers(ctx context.Context, resources domain.ContainerResources) error {
	_ = ctx
	_ = resources
	return nil
}
