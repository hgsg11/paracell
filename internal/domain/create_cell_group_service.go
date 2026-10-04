package domain

// CellGroupIDGenerator allocates identities for the group and its runtime Cells.
type CellGroupIDGenerator interface {
	NewID() string
}

// CreateCellGroupService constructs independent runtime Cells from template data.
func CreateCellGroupService(cfg Templates, resolved ResolvedTemplate, issue, commanderID string, ids CellGroupIDGenerator) (CommanderCell, []TargetCell, []DependencyCell, error) {
	spec := *resolved.Commander
	group, err := NewCellGroup(ids.NewID(), issue, cfg.ProjectName, resolved.Name, cfg.SourceDriverType, cfg.ContainerDriverType, cfg.NotificationDriverType)
	if err != nil {
		return CommanderCell{}, nil, nil, err
	}
	dependencyIDs := make(map[string]string, len(spec.Dependencies))
	dependencies := make([]DependencyCell, 0, len(spec.Dependencies))
	for _, dependencySpec := range spec.Dependencies {
		id := ids.NewID()
		container, err := NewContainer(nil, dependencySpec.Container.Name, dependencySpec.Container.Mode)
		if err != nil {
			return CommanderCell{}, nil, nil, err
		}
		dependency, err := NewDependencyCell(id, group.ID, dependencySpec.Name, container)
		if err != nil {
			return CommanderCell{}, nil, nil, err
		}
		dependencyIDs[dependencySpec.Name] = id
		dependencies = append(dependencies, dependency)
	}
	targets := make([]TargetCell, 0, len(spec.Targets))
	for _, targetSpec := range spec.Targets {
		id := ids.NewID()
		var source *Source
		if targetSpec.Source != nil {
			value, err := BuildSource(*targetSpec.Source, issue)
			if err != nil {
				return CommanderCell{}, nil, nil, err
			}
			source = &value
		}
		var container *Container
		if targetSpec.Container != nil {
			value, err := NewContainer(nil, targetSpec.Container.Name, targetSpec.Container.Mode)
			if err != nil {
				return CommanderCell{}, nil, nil, err
			}
			container = &value
		}
		dependencies := make([]string, 0, len(targetSpec.Dependencies))
		for _, dependencyName := range targetSpec.Dependencies {
			dependencies = append(dependencies, dependencyIDs[dependencyName])
		}
		target, err := NewTargetCell(id, group.ID, targetSpec.Name, source, container, dependencies)
		if err != nil {
			return CommanderCell{}, nil, nil, err
		}
		targets = append(targets, target)
	}
	windows := make([]WorkspaceWindow, 0, len(spec.Workspace.Windows))
	for _, item := range spec.Workspace.Windows {
		window, err := NewWorkspaceWindow(item.Name, item.Command)
		if err != nil {
			return CommanderCell{}, nil, nil, err
		}
		windows = append(windows, window)
	}
	workspace := NewWorkspace(cfg.WorkspaceDriverType, windows)
	commander, err := NewCommanderCell(commanderID, &group, workspace)
	if err != nil {
		return CommanderCell{}, nil, nil, err
	}
	return commander, targets, dependencies, nil
}
