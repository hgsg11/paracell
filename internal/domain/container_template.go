package domain

import (
	"fmt"
)

type ContainerTemplate struct {
	Name         string
	Mode         Mode
	Environments []Environment
	Mounts       []Mount
}

func NewContainerTemplate(name string, mode Mode, environments []Environment, mounts []Mount) (ContainerTemplate, error) {
	if name == "" {
		return ContainerTemplate{}, fmt.Errorf("container name is required")
	}
	validatedMode, err := NewMode(string(mode))
	if err != nil {
		return ContainerTemplate{}, err
	}
	if validatedMode == Dependency && (len(environments) != 0 || len(mounts) != 0) {
		return ContainerTemplate{}, fmt.Errorf("dependency container %q cannot define environments or mounts", name)
	}
	return ContainerTemplate{Name: name, Mode: validatedMode, Environments: append([]Environment(nil), environments...), Mounts: append([]Mount(nil), mounts...)}, nil
}
