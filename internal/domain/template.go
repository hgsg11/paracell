package domain

import "fmt"

// Template groups the peer Cell specifications used to create one CellGroup.
type Template struct {
	Name         string
	Extends      string
	Abstract     bool
	Commander    *CommanderCellSpec
	Targets      []TargetCellSpec
	Dependencies []DependencyCellSpec
}

func (t Template) resolve(vars TemplateVars) (ResolvedTemplate, error) {
	if t.Commander == nil {
		return ResolvedTemplate{}, fmt.Errorf("template %q does not define a CommanderCell", t.Name)
	}
	commander := t.Commander
	workspace, err := commander.Workspace.render(vars)
	if err != nil {
		return ResolvedTemplate{}, err
	}
	targets := make([]TargetCellSpec, 0, len(t.Targets))
	for _, target := range t.Targets {
		containers := make([]ContainerTemplate, 0, len(target.Containers))
		for _, container := range target.Containers {
			rendered, err := container.render(vars)
			if err != nil {
				return ResolvedTemplate{}, err
			}
			containers = append(containers, rendered)
		}
		rendered, err := NewTargetCellSpec(target.Name, target.Source, containers)
		if err != nil {
			return ResolvedTemplate{}, err
		}
		targets = append(targets, rendered)
	}
	dependencies := make([]DependencyCellSpec, 0, len(t.Dependencies))
	for _, dependency := range t.Dependencies {
		rendered, err := NewDependencyCellSpec(dependency.Name)
		if err != nil {
			return ResolvedTemplate{}, err
		}
		dependencies = append(dependencies, rendered)
	}
	resolvedCommander, err := NewCommanderCellSpec(commander.Name, workspace)
	if err != nil {
		return ResolvedTemplate{}, err
	}
	return NewResolvedTemplate(t.Name, resolvedCommander, targets, dependencies), nil
}

func (t Template) merge(parent Template) (Template, error) {
	commander, targets, dependencies := parent.Commander, parent.Targets, parent.Dependencies
	if t.Commander != nil {
		commander = t.Commander
	}
	if t.Targets != nil {
		targets = t.Targets
	}
	if t.Dependencies != nil {
		dependencies = t.Dependencies
	}
	return NewUnresolvedTemplate(t.Name, t.Extends, t.Abstract, commander, targets, dependencies)
}

func NewUnresolvedTemplate(name, extends string, abstract bool, commander *CommanderCellSpec, targets []TargetCellSpec, dependencies []DependencyCellSpec) (Template, error) {
	if name == "" {
		return Template{}, fmt.Errorf("template name is required")
	}
	var copy *CommanderCellSpec
	if commander != nil {
		cloned, err := cloneCommanderCellSpec(*commander)
		if err != nil {
			return Template{}, err
		}
		copy = &cloned
	}
	targetNames := make(map[string]struct{}, len(targets))
	resourceNames := make(map[string]string, len(targets))
	var targetCopies []TargetCellSpec
	for _, target := range targets {
		validated, err := NewTargetCellSpec(target.Name, target.Source, target.Containers)
		if err != nil {
			return Template{}, err
		}
		if _, exists := targetNames[validated.Name]; exists {
			return Template{}, fmt.Errorf("duplicate target cell %q", validated.Name)
		}
		targetNames[validated.Name] = struct{}{}
		resourceName := SafeResourceName(validated.Name, "target")
		if existing, exists := resourceNames[resourceName]; exists {
			return Template{}, fmt.Errorf("target cells %q and %q resolve to the same source path", existing, validated.Name)
		}
		resourceNames[resourceName] = validated.Name
		if validated.Source != nil {
			source := *validated.Source
			validated.Source = &source
		}
		for i := range validated.Containers {
			for j := 0; j < i; j++ {
				if validated.Containers[i].Name == validated.Containers[j].Name {
					return Template{}, fmt.Errorf("duplicate container %q in target cell %q", validated.Containers[i].Name, validated.Name)
				}
			}
			validated.Containers[i].Environments = append([]Environment(nil), validated.Containers[i].Environments...)
			validated.Containers[i].Mounts = append([]Mount(nil), validated.Containers[i].Mounts...)
		}
		targetCopies = append(targetCopies, validated)
	}
	dependencyNames := make(map[string]struct{}, len(dependencies))
	var dependencyCopies []DependencyCellSpec
	for _, dependency := range dependencies {
		validated, err := NewDependencyCellSpec(dependency.Name)
		if err != nil {
			return Template{}, err
		}
		if _, exists := dependencyNames[validated.Name]; exists {
			return Template{}, fmt.Errorf("duplicate dependency cell %q", validated.Name)
		}
		dependencyNames[validated.Name] = struct{}{}
		dependencyCopies = append(dependencyCopies, validated)
	}
	return Template{Name: name, Extends: extends, Abstract: abstract, Commander: copy,
		Targets: targetCopies, Dependencies: dependencyCopies}, nil
}
