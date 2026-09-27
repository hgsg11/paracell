package container

import (
	"context"
	"testing"

	"github.com/hgsg11/paracell/internal/domain"
)

func TestNoopAdapterはContainer作成で何もしない(t *testing.T) {
	adapter := NoopAdapter{}
	if err := adapter.CreateContainerNetwork(context.Background(), ""); err != nil {
		t.Fatalf("CreateContainerNetwork error = %v, want nil", err)
	}
	if _, err := adapter.CreateContainer(context.Background(), "", domain.Target, nil, nil, "", "", "", ""); err != nil {
		t.Fatalf("CreateContainer error = %v, want nil", err)
	}
}

func TestNoopAdapterはCleanContainersで何もしない(t *testing.T) {
	err := NoopAdapter{}.CleanContainers(context.Background(), "", nil, nil)

	if err != nil {
		t.Fatalf("CleanContainers error = %v, want nil", err)
	}
}
