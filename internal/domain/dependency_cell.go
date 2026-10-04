package domain

import "fmt"

// DependencyCell is one independent container-backed runtime dependency.
type DependencyCell struct {
	ID          string
	CellGroupID string
	Name        string
	Container   Container
}

func NewDependencyCell(id, cellGroupID, name string, container Container) (DependencyCell, error) {
	if id == "" || cellGroupID == "" || name == "" {
		return DependencyCell{}, fmt.Errorf("dependency cell id, CellGroup reference, and name are required")
	}
	if container.Mode != Dependency || container.SourceContainer == "" {
		return DependencyCell{}, fmt.Errorf("dependency cell %q requires one dependency container", name)
	}
	container.Network = append([]string(nil), container.Network...)
	return DependencyCell{ID: id, CellGroupID: cellGroupID, Name: name, Container: container}, nil
}

func RestoreDependencyCell(stored DependencyCell) (DependencyCell, error) {
	container, err := NewContainer(stored.Container.Network, stored.Container.SourceContainer, stored.Container.Mode)
	if err != nil {
		return DependencyCell{}, err
	}
	return NewDependencyCell(stored.ID, stored.CellGroupID, stored.Name, container)
}
