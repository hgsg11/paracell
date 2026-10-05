package domain

import "fmt"

// TargetCellSpec defines one target runtime Cell without dependency links.
type TargetCellSpec struct {
	Name       string
	Source     *SourceTemplate
	Containers []ContainerTemplate
}

func NewTargetCellSpec(name string, source *SourceTemplate, containers []ContainerTemplate) (TargetCellSpec, error) {
	if name == "" {
		return TargetCellSpec{}, fmt.Errorf("target cell name is required")
	}
	if source == nil && len(containers) == 0 {
		return TargetCellSpec{}, fmt.Errorf("target cell %q requires a source or container", name)
	}
	for _, container := range containers {
		if container.Mode != Target {
			return TargetCellSpec{}, fmt.Errorf("target cell %q requires target containers", name)
		}
	}
	return TargetCellSpec{Name: name, Source: source, Containers: append([]ContainerTemplate(nil), containers...)}, nil
}
