package domain

import "fmt"

// Template is the aggregate root for a CommanderCell and its related Cell specs.
type Template struct {
	Name      string
	Extends   string
	Abstract  bool
	Commander *CommanderCellSpec
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
	targets := make([]TargetCellSpec, 0, len(commander.Targets))
	for _, target := range commander.Targets {
		var container *ContainerTemplate
		if target.Container != nil {
			rendered, err := target.Container.render(vars)
			if err != nil {
				return ResolvedTemplate{}, err
			}
			container = &rendered
		}
		rendered, err := NewTargetCellSpec(target.Name, target.Source, container, target.Dependencies)
		if err != nil {
			return ResolvedTemplate{}, err
		}
		targets = append(targets, rendered)
	}
	dependencies := make([]DependencyCellSpec, 0, len(commander.Dependencies))
	for _, dependency := range commander.Dependencies {
		container, err := dependency.Container.render(vars)
		if err != nil {
			return ResolvedTemplate{}, err
		}
		rendered, err := NewDependencyCellSpec(dependency.Name, container)
		if err != nil {
			return ResolvedTemplate{}, err
		}
		dependencies = append(dependencies, rendered)
	}
	resolvedCommander, err := NewCommanderCellSpec(commander.Name, workspace, targets, dependencies)
	if err != nil {
		return ResolvedTemplate{}, err
	}
	return NewResolvedTemplate(t.Name, resolvedCommander), nil
}

func (t Template) merge(parent Template) (Template, error) {
	commander := parent.Commander
	if t.Commander != nil {
		commander = t.Commander
	}
	return NewUnresolvedTemplate(t.Name, t.Extends, t.Abstract, commander)
}

func NewTemplate(name string, commander CommanderCellSpec) (Template, error) {
	return NewUnresolvedTemplate(name, "", false, &commander)
}

func NewUnresolvedTemplate(name, extends string, abstract bool, commander *CommanderCellSpec) (Template, error) {
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
	return Template{Name: name, Extends: extends, Abstract: abstract, Commander: copy}, nil
}
