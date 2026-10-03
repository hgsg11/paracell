package domain

import "fmt"

// TargetCellSpec is the Template-owned definition of one target runtime Cell.
type TargetCellSpec struct {
	Name         string
	Source       *SourceTemplate
	Container    *ContainerTemplate
	Dependencies []string
}

func NewTargetCellSpec(name string, source *SourceTemplate, container *ContainerTemplate, dependencies []string) (TargetCellSpec, error) {
	if name == "" {
		return TargetCellSpec{}, fmt.Errorf("target cell name is required")
	}
	if source == nil && container == nil {
		return TargetCellSpec{}, fmt.Errorf("target cell %q requires a source or container", name)
	}
	return TargetCellSpec{Name: name, Source: source, Container: container, Dependencies: append([]string(nil), dependencies...)}, nil
}
