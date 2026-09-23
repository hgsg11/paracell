package domain

import "context"

type ContainerCreationPort interface {
	CreateContainers(ctx context.Context, templates []ContainerTemplate, cellName string, project string, network string, sourcePath string) (map[string][]string, error)
}

func CreateContainersService(ctx context.Context, templates []ContainerTemplate, cellName string, project string, network string, sourcePath string, port ContainerCreationPort) (map[string][]string, error) {
	return port.CreateContainers(ctx, templates, cellName, project, network, sourcePath)
}
