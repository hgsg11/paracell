package domain

import "fmt"

// DependencyCell is one independent container-backed runtime dependency.
type DependencyCell struct {
	ID          string
	CommanderID string
	Name        string
	Container   Container
}

func NewDependencyCell(id, commanderID, name string, container Container) (DependencyCell, error) {
	if id == "" || commanderID == "" || name == "" {
		return DependencyCell{}, fmt.Errorf("dependency cell id, CommanderCell reference, and name are required")
	}
	if container.Mode != Dependency || container.SourceContainer == "" {
		return DependencyCell{}, fmt.Errorf("dependency cell %q requires one dependency container", name)
	}
	container.Network = append([]string(nil), container.Network...)
	return DependencyCell{ID: id, CommanderID: commanderID, Name: name, Container: container}, nil
}

func RestoreDependencyCell(stored DependencyCell) (DependencyCell, error) {
	container, err := NewContainer(stored.Container.Network, stored.Container.SourceContainer, stored.Container.Mode)
	if err != nil {
		return DependencyCell{}, err
	}
	return NewDependencyCell(stored.ID, stored.CommanderID, stored.Name, container)
}
