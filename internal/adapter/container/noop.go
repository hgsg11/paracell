package container

import (
	"context"

	"github.com/hgsg11/paracell/internal/domain"
)

type NoopAdapter struct{}

func NewNoopAdapter() NoopAdapter { return NoopAdapter{} }

func (a NoopAdapter) CreateContainerNetwork(_ context.Context, _ string) error { return nil }

func (a NoopAdapter) CreateContainer(_ context.Context, _ string, _ domain.Mode, _ []domain.Environment, _ []domain.Mount, _, _, _, _ string) ([]string, error) {
	return nil, nil
}

func (a NoopAdapter) CleanContainers(_ context.Context, _ string, _, _ []string) error {
	return nil
}
