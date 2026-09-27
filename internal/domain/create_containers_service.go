package domain

import "context"

type ContainerCreationPort interface {
	CreateContainerNetwork(ctx context.Context, network string) error
	CreateContainer(ctx context.Context, containerName string, environments []Environment, mounts []Mount, cellName string, project string, network string, sourcePath string) ([]string, error)
	ConnectDependency(ctx context.Context, containerName string, network string) ([]string, error)
}

func CreateContainersService(ctx context.Context, cell *Cell, port ContainerCreationPort) (map[string][]string, error) {
	_, cellName, project, _, _ := cell.SessionPreparation()
	network := cell.ContainerNetworkName()
	if err := port.CreateContainerNetwork(ctx, network); err != nil {
		return nil, err
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
			return nil, err
		}
		networks[container.SourceContainer] = containerNetworks
	}
	return networks, nil
}
