package domain

import "context"

type ContainerPort interface {
	CreateContainers(context.Context, ContainerResources) (map[string][]string, error)
	CleanContainers(context.Context, ContainerResources) error
}
