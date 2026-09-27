package container

import (
	"context"
	"testing"
)

func TestNoopAdapterはContainer作成で何もしない(t *testing.T) {
	adapter := NoopAdapter{}
	if err := adapter.CreateContainerNetwork(context.Background(), ""); err != nil {
		t.Fatalf("CreateContainerNetwork error = %v, want nil", err)
	}
	if _, err := adapter.CreateContainer(context.Background(), "", nil, nil, "", "", "", ""); err != nil {
		t.Fatalf("CreateContainer error = %v, want nil", err)
	}
	if _, err := adapter.ConnectDependency(context.Background(), "", ""); err != nil {
		t.Fatalf("ConnectDependency error = %v, want nil", err)
	}
}

func TestNoopAdapterはCleanContainersで何もしない(t *testing.T) {
	err := NoopAdapter{}.CleanContainers(context.Background(), "", nil, nil)

	if err != nil {
		t.Fatalf("CleanContainers error = %v, want nil", err)
	}
}
