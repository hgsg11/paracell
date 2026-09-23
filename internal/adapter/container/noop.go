package container

import (
	"context"

	"github.com/hgsg11/paracell/internal/domain"
)

type NoopAdapter struct{}

func NewNoopAdapter() NoopAdapter { return NoopAdapter{} }

func (a NoopAdapter) CreateContainers(_ context.Context, _ []domain.ContainerTemplate, _, _, _, _ string) (map[string][]string, error) {
	return map[string][]string{}, nil
}

func (a NoopAdapter) CleanContainers(_ context.Context, _ string, _, _ []string) error {
	return nil
}
