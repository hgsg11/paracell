package domain

import "fmt"

// TargetCell is one independent runtime development target, not an aggregate root.
type TargetCell struct {
	ID           string
	CommanderID  string
	Name         string
	Source       *Source
	Container    *Container
	Dependencies []string
}

func NewTargetCell(id, commanderID, name string, source *Source, container *Container, dependencies []string) (TargetCell, error) {
	if id == "" || commanderID == "" || name == "" {
		return TargetCell{}, fmt.Errorf("target cell id, CommanderCell reference, and name are required")
	}
	if source == nil && container == nil {
		return TargetCell{}, fmt.Errorf("target cell %q requires a source or container", name)
	}
	if container != nil && container.Mode != Target {
		return TargetCell{}, fmt.Errorf("target cell %q requires a target container", name)
	}
	var sourceCopy *Source
	if source != nil {
		copy := *source
		sourceCopy = &copy
	}
	var containerCopy *Container
	if container != nil {
		copy := *container
		copy.Network = append([]string(nil), container.Network...)
		containerCopy = &copy
	}
	return TargetCell{ID: id, CommanderID: commanderID, Name: name, Source: sourceCopy, Container: containerCopy, Dependencies: append([]string(nil), dependencies...)}, nil
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
	var container *Container
	if stored.Container != nil {
		validated, err := NewContainer(stored.Container.Network, stored.Container.SourceContainer, stored.Container.Mode)
		if err != nil {
			return TargetCell{}, err
		}
		container = &validated
	}
	return NewTargetCell(stored.ID, stored.CommanderID, stored.Name, source, container, stored.Dependencies)
}
