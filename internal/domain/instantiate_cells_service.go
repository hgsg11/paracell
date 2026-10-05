package domain

// RuntimeCellIDGenerator allocates identities for runtime Cells.
type RuntimeCellIDGenerator interface {
	NewID() string
}

// InstantiateCellsService constructs independent runtime Cells from template data.
func InstantiateCellsService(group *CellGroup, commanderSpec CommanderCellSpec, targetSpecs []TargetCellSpec, dependencySpecs []DependencyCellSpec, workspaceDriver WorkspaceDriverType, commanderID string, ids RuntimeCellIDGenerator) (CommanderCell, []TargetCell, []DependencyCell, error) {
	dependencies := make([]DependencyCell, 0, len(dependencySpecs))
	for _, dependencySpec := range dependencySpecs {
		id := ids.NewID()
		container, err := NewContainer(nil, dependencySpec.Name, Dependency)
		if err != nil {
			return CommanderCell{}, nil, nil, err
		}
		dependency, err := NewDependencyCell(id, group.ID, dependencySpec.Name, container)
		if err != nil {
			return CommanderCell{}, nil, nil, err
		}
		dependencies = append(dependencies, dependency)
	}
	targets := make([]TargetCell, 0, len(targetSpecs))
	for _, targetSpec := range targetSpecs {
		id := ids.NewID()
		var source *Source
		if targetSpec.Source != nil {
			value, err := BuildSource(*targetSpec.Source, group.Issue)
			if err != nil {
				return CommanderCell{}, nil, nil, err
			}
			source = &value
		}
		containers := make([]*Container, 0, len(targetSpec.Containers))
		for _, containerSpec := range targetSpec.Containers {
			value, err := NewContainer(nil, containerSpec.Name, containerSpec.Mode)
			if err != nil {
				return CommanderCell{}, nil, nil, err
			}
			containers = append(containers, &value)
		}
		target, err := NewTargetCell(id, group.ID, targetSpec.Name, source, containers)
		if err != nil {
			return CommanderCell{}, nil, nil, err
		}
		targets = append(targets, target)
	}
	windows := make([]WorkspaceWindow, 0, len(commanderSpec.Workspace.Windows))
	for _, item := range commanderSpec.Workspace.Windows {
		window, err := NewWorkspaceWindow(item.Name, item.Command)
		if err != nil {
			return CommanderCell{}, nil, nil, err
		}
		windows = append(windows, window)
	}
	workspace := NewWorkspace(workspaceDriver, windows)
	commander, err := NewCommanderCell(commanderID, group, workspace)
	if err != nil {
		return CommanderCell{}, nil, nil, err
	}
	return commander, targets, dependencies, nil
}
