package domain

import "fmt"

// DependencyCellSpec defines the required container for a dependency Cell.
type DependencyCellSpec struct {
	Name      string
	Container ContainerTemplate
}

func NewDependencyCellSpec(name string, container ContainerTemplate) (DependencyCellSpec, error) {
	if name == "" {
		return DependencyCellSpec{}, fmt.Errorf("dependency cell name is required")
	}
	if container.Mode != Dependency {
		return DependencyCellSpec{}, fmt.Errorf("dependency cell %q requires a dependency container", name)
	}
	return DependencyCellSpec{Name: name, Container: container}, nil
}
