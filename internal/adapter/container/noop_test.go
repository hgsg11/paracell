package container

import (
	"context"
	"testing"

	"github.com/shige1114/paradev/internal/domain"
)

func TestNoopAdapterはCreateContainersで何もしない(t *testing.T) {
	err := NoopAdapter{}.CreateContainers(context.Background(), domain.Cell{}, domain.Template{})

	if err != nil {
		t.Fatalf("CreateContainers error = %v, want nil", err)
	}
}

func TestNoopAdapterはRemoveContainersで何もしない(t *testing.T) {
	err := NoopAdapter{}.RemoveContainers(context.Background(), domain.Cell{})

	if err != nil {
		t.Fatalf("RemoveContainers error = %v, want nil", err)
	}
}
