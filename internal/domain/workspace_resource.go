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
