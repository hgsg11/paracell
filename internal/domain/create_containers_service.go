package domain

import (
	"context"
	"fmt"
)

type ContainerCreationPort interface {
	CreateContainerNetwork(ctx context.Context, network string) error
	CreateContainer(ctx context.Context, containerName string, environments []Environment, mounts []Mount, cellName string, project string, network string, sourcePath string) ([]string, error)
	ConnectDependency(ctx context.Context, containerName string, network string) ([]string, error)
}

type ContainerCellSavePort interface {
	SaveCell(ctx context.Context, cell Cell) error
}

func CreateContainersService(ctx context.Context, cell *Cell, port ContainerCreationPort, cells ContainerCellSavePort) error {
	fail := func(err error) error {
		if stateErr := cell.FailCreation(CreationStageContainers, err); stateErr != nil {
			return fmt.Errorf("%w; record container creation failure: %v", err, stateErr)
		}
		if saveErr := cells.SaveCell(ctx, *cell); saveErr != nil {
			return fmt.Errorf("%w; save container creation failure: %v", err, saveErr)
		}
		return err
	}
	if err := cell.SetCreationStage(CreationStageContainers); err != nil {
		return err
	}
	if err := cells.SaveCell(ctx, *cell); err != nil {
		return err
	}
	_, cellName, project, _, _ := cell.SessionPreparation()
	network := cell.ContainerNetworkName()
	if err := port.CreateContainerNetwork(ctx, network); err != nil {
		return fail(err)
	}

	networks := make(map[string][]string, len(cell.Containers.Items))
	for _, container := range cell.Containers.Items {
		var containerNetworks []string
		var err error
		if container.Mode == Dependency {
			containerNetworks, err = port.ConnectDependency(ctx, container.SourceContainer, network)
		} else {
			containerNetworks, err = port.CreateContainer(ctx, container.SourceContainer, container.Environments, container.Mounts, cellName, project, network, cell.WorkingDirectory())
		}
		if err != nil {
			return fail(err)
		}
		networks[container.SourceContainer] = containerNetworks
	}
	if err := cell.RecordContainerNetworks(networks); err != nil {
		return err
	}
	return cells.SaveCell(ctx, *cell)
}
