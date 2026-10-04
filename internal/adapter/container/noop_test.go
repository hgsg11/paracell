package container

import (
	"context"
	"testing"

	"github.com/hgsg11/paracell/internal/domain"
)

func TestNoopAdapterはCreateContainersで何もしない(t *testing.T) {
	_, err := NoopAdapter{}.CreateContainers(context.Background(), domain.ContainerResources{})

	if err != nil {
		t.Fatalf("CreateContainers error = %v, want nil", err)
	}
}

func TestNoopAdapterはCleanContainersで何もしない(t *testing.T) {
	err := NoopAdapter{}.CleanContainers(context.Background(), domain.ContainerResources{})

	if err != nil {
		t.Fatalf("CleanContainers error = %v, want nil", err)
	}
}
