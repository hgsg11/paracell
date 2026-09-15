package domain

import (
	"context"
)

func CreateContainers(ctx context.Context, cell *Cell, templates []ContainerTemplate, port ContainerPort) error {
	networks, err := port.CreateContainers(ctx, cell.containerResources(templates))
	if err != nil {
		return err
	}
	cell.RecordContainerNetworks(networks)
	return nil
}

func CleanContainers(ctx context.Context, cell Cell, port ContainerPort) error {
	return port.CleanContainers(ctx, cell.containerResources(nil))
}
