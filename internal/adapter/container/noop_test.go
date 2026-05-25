package container

import (
	"context"
	"testing"

	"github.com/hgsg11/paracell/internal/domain"
)

func TestNoopAdapterはCreateContainersで何もしない(t *testing.T) {
	err := NoopAdapter{}.CreateContainers(context.Background(), domain.Cell{}, domain.Template{})

	if err != nil {
		t.Fatalf("CreateContainers error = %v, want nil", err)
	}
}

func TestNoopAdapterはCleanContainersで何もしない(t *testing.T) {
	err := NoopAdapter{}.CleanContainers(context.Background(), domain.Cell{})

	if err != nil {
		t.Fatalf("CleanContainers error = %v, want nil", err)
	}
}
