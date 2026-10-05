package domain

import "fmt"

// TargetCell is one independent runtime development target, not an aggregate root.
type TargetCell struct {
	ID          string
	CellGroupID string
	Name        string
	Source      *Source
	Containers  []*Container
}

func NewTargetCell(id, cellGroupID, name string, source *Source, containers []*Container) (TargetCell, error) {
	if id == "" || cellGroupID == "" || name == "" {
		return TargetCell{}, fmt.Errorf("target cell id, CellGroup reference, and name are required")
	}
	if source == nil && len(containers) == 0 {
		return TargetCell{}, fmt.Errorf("target cell %q requires a source or container", name)
	}
	for _, container := range containers {
		if container == nil {
			return TargetCell{}, fmt.Errorf("target cell %q has a nil container", name)
		}
		if container.Mode != Target {
			return TargetCell{}, fmt.Errorf("target cell %q requires target containers", name)
		}
	}
	var sourceCopy *Source
	if source != nil {
		copy := *source
		sourceCopy = &copy
	}
	containerCopies := make([]*Container, len(containers))
	for i, container := range containers {
		copy := *container
		copy.Network = append([]string(nil), container.Network...)
		containerCopies[i] = &copy
	}
	return TargetCell{ID: id, CellGroupID: cellGroupID, Name: name, Source: sourceCopy, Containers: containerCopies}, nil
}

func RestoreTargetCell(stored TargetCell) (TargetCell, error) {
	var source *Source
	if stored.Source != nil {
		validated, err := NewSource(stored.Source.Path, stored.Source.Base, stored.Source.Branch)
		if err != nil {
			return TargetCell{}, err
		}
		source = &validated
	}
	containers := make([]*Container, len(stored.Containers))
	for i, storedContainer := range stored.Containers {
		validated, err := NewContainer(storedContainer.Network, storedContainer.SourceContainer, storedContainer.Mode)
		if err != nil {
			return TargetCell{}, err
		}
		containers[i] = &validated
	}
	return NewTargetCell(stored.ID, stored.CellGroupID, stored.Name, source, containers)
}
