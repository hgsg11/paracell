package domain

import "fmt"

// DependencyCellSpec defines the required container for a dependency Cell.
type DependencyCellSpec struct {
	Name string
}

func NewDependencyCellSpec(name string) (DependencyCellSpec, error) {
	if name == "" {
		return DependencyCellSpec{}, fmt.Errorf("dependency cell name is required")
	}
	return DependencyCellSpec{Name: name}, nil
}
