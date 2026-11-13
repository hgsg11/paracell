package domain

import (
	"fmt"
	"path/filepath"
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
	merged, err := mergeMounts(name, mounts)
	if err != nil {
		return ContainerTemplate{}, err
	}
	return ContainerTemplate{Name: name, Mode: validatedMode, Environments: append([]Environment(nil), environments...), Mounts: merged}, nil
}

// mergeMounts keeps one mount per target path. Paths that become equal once
// cleaned are merged when they mount the same source.
func mergeMounts(container string, mounts []Mount) ([]Mount, error) {
	merged := make([]Mount, 0, len(mounts))
	sources := make(map[string]string, len(mounts))
	for _, mount := range mounts {
		target := filepath.Clean(mount.TargetPath)
		if source, exists := sources[target]; exists {
			if source != mount.SourcePath {
				return nil, fmt.Errorf("container %q mounts different sources at %q", container, target)
			}
			continue
		}
		sources[target] = mount.SourcePath
		merged = append(merged, Mount{TargetPath: target, SourcePath: mount.SourcePath})
	}
	return merged, nil
}

func (c ContainerTemplate) render(vars TemplateVars) (ContainerTemplate, error) {
	environments := make([]Environment, 0, len(c.Environments))
	for _, environment := range c.Environments {
		value, err := vars.Render(environment.Value)
		if err != nil {
			return ContainerTemplate{}, fmt.Errorf("render environment %q for container %q: %w", environment.Name, c.Name, err)
		}
		rendered, err := NewEnvironment(environment.Name, value)
		if err != nil {
			return ContainerTemplate{}, err
		}
		environments = append(environments, rendered)
	}
	return NewContainerTemplate(c.Name, c.Mode, environments, c.Mounts)
}
