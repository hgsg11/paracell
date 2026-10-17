package domain

type WorkspaceResource struct {
	Name             string
	CellName         string
	Project          string
	DisplayLabel     string
	WorkingDirectory string
	Windows          []WorkspaceWindow
}

func NewWorkspaceResource(name string, cellName string, project string, displayLabel string, workingDirectory string, windows []WorkspaceWindow) WorkspaceResource {
	return WorkspaceResource{Name: name, CellName: cellName, Project: project, DisplayLabel: displayLabel, WorkingDirectory: workingDirectory, Windows: append([]WorkspaceWindow(nil), windows...)}
}

// WorkspaceSocketName returns the dedicated tmux socket name for the cell
// sessions of a project, which keeps them out of the default server's lists.
func WorkspaceSocketName(project string) string {
	return "paracell-" + SafeResourceName(project, "project")
}

func (r WorkspaceResource) SocketName() string {
	return WorkspaceSocketName(r.Project)
}

// RootWorkspaceName is the name of the project's root session on the default tmux server.
func RootWorkspaceName(project string) string {
	return project + "-root"
}
