package domain

// BuildWorkspaceResourceService builds the workspace resource of the group's
// CommanderCell. A Cell without any source-backed Target has no Cell
// directory, so its workspace starts at the project root instead.
func BuildWorkspaceResourceService(group CellGroup, workspace Workspace, targets []TargetCell) WorkspaceResource {
	resource := group.WorkspaceResource(workspace)
	for _, target := range targets {
		if target.Source != nil {
			return resource
		}
	}
	return NewWorkspaceResource(resource.Name, resource.CellName, resource.Project, resource.DisplayLabel, ".", resource.Windows)
}
