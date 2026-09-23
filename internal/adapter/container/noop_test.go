package container

import (
	"context"
	"testing"
)

func TestNoopAdapterはCreateContainersで何もしない(t *testing.T) {
	_, err := NoopAdapter{}.CreateContainers(context.Background(), nil, "", "", "", "")

	if err != nil {
		t.Fatalf("CreateContainers error = %v, want nil", err)
	}
}

func TestNoopAdapterはCleanContainersで何もしない(t *testing.T) {
	err := NoopAdapter{}.CleanContainers(context.Background(), "", nil, nil)

	if err != nil {
		t.Fatalf("CleanContainers error = %v, want nil", err)
	}
}
