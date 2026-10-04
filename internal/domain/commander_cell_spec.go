package domain

import "fmt"

// CommanderCellSpec defines a Workspace and the target/dependency Cells for a Template.
type CommanderCellSpec struct {
	Name         string
	Workspace    WorkspaceTemplate
	Targets      []TargetCellSpec
	Dependencies []DependencyCellSpec
}

func NewCommanderCellSpec(name string, workspace WorkspaceTemplate, targets []TargetCellSpec, dependencies []DependencyCellSpec) (CommanderCellSpec, error) {
	if name == "" {
		return CommanderCellSpec{}, fmt.Errorf("commander cell name is required")
	}
	dependencyNames := make(map[string]struct{}, len(dependencies))
	for _, dependency := range dependencies {
		if dependency.Name == "" || dependency.Container.Mode != Dependency {
			return CommanderCellSpec{}, fmt.Errorf("dependency cell name and container are required")
		}
		if _, exists := dependencyNames[dependency.Name]; exists {
			return CommanderCellSpec{}, fmt.Errorf("duplicate dependency cell %q", dependency.Name)
		}
		dependencyNames[dependency.Name] = struct{}{}
	}
	targetNames := make(map[string]struct{}, len(targets))
	targetResourceNames := make(map[string]string, len(targets))
	for _, target := range targets {
		if target.Name == "" || (target.Source == nil && target.Container == nil) {
			return CommanderCellSpec{}, fmt.Errorf("target cell name and a source or container are required")
		}
		if target.Container != nil && target.Container.Mode != Target {
			return CommanderCellSpec{}, fmt.Errorf("target cell %q requires a target container", target.Name)
		}
		if _, exists := targetNames[target.Name]; exists {
			return CommanderCellSpec{}, fmt.Errorf("duplicate target cell %q", target.Name)
		}
		targetNames[target.Name] = struct{}{}
		resourceName := SafeResourceName(target.Name, "target")
		if existing, exists := targetResourceNames[resourceName]; exists {
			return CommanderCellSpec{}, fmt.Errorf("target cells %q and %q resolve to the same source path", existing, target.Name)
		}
		targetResourceNames[resourceName] = target.Name
		for _, dependency := range target.Dependencies {
			if _, exists := dependencyNames[dependency]; !exists {
				return CommanderCellSpec{}, fmt.Errorf("target cell %q references unknown dependency cell %q", target.Name, dependency)
			}
		}
	}
	targetsCopy := append([]TargetCellSpec(nil), targets...)
	for index := range targetsCopy {
		current := targets[index]
		targetsCopy[index].Dependencies = append([]string(nil), current.Dependencies...)
		if current.Source != nil {
			source := *current.Source
			targetsCopy[index].Source = &source
		}
		if current.Container != nil {
			container := *current.Container
			container.Environments = append([]Environment(nil), current.Container.Environments...)
			container.Mounts = append([]Mount(nil), current.Container.Mounts...)
			targetsCopy[index].Container = &container
		}
	}
	dependenciesCopy := append([]DependencyCellSpec(nil), dependencies...)
	for index := range dependenciesCopy {
		dependenciesCopy[index].Container.Environments = append([]Environment(nil), dependencies[index].Container.Environments...)
		dependenciesCopy[index].Container.Mounts = append([]Mount(nil), dependencies[index].Container.Mounts...)
	}
	return CommanderCellSpec{
		Name: name, Workspace: NewWorkspaceTemplate(workspace.Windows),
		Targets: targetsCopy, Dependencies: dependenciesCopy,
	}, nil
}

func cloneCommanderCellSpec(spec CommanderCellSpec) (CommanderCellSpec, error) {
	return NewCommanderCellSpec(spec.Name, spec.Workspace, spec.Targets, spec.Dependencies)
}
