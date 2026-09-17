package domain

import (
	"context"
)

func CreateContainers(ctx context.Context, cell *Cell, driver ContainerDriverType, templates []ContainerTemplate, port ContainerPort) error {
	containers, err := buildContainers(driver, templates)
	if err != nil {
		return err
	}
	cell.PlanContainers(containers)
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
