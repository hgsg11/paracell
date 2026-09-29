package domain

import "fmt"

type Container struct {
	Network         []string
	SourceContainer string
	Mode            Mode
	Environments    []Environment
	Mounts          []Mount
}

func NewContainer(network []string, sourceContainer string, mode Mode, environments []Environment, mounts []Mount) (Container, error) {
	if sourceContainer == "" {
		return Container{}, fmt.Errorf("source container is required")
	}
	validatedMode, err := NewMode(string(mode))
	if err != nil {
		return Container{}, err
	}
	if validatedMode == Dependency && (len(environments) != 0 || len(mounts) != 0) {
		return Container{}, fmt.Errorf("dependency container %q cannot define environments or mounts", sourceContainer)
	}
	return Container{Network: append([]string(nil), network...), SourceContainer: sourceContainer, Mode: validatedMode, Environments: append([]Environment(nil), environments...), Mounts: append([]Mount(nil), mounts...)}, nil
}
