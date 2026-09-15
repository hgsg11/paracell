package domain

type SessionResource struct {
	Name             string
	CellName         string
	Project          string
	DisplayLabel     string
	WorkingDirectory string
	Windows          []SessionWindow
}

func NewSessionResource(name string, cellName string, project string, displayLabel string, workingDirectory string, windows []SessionWindow) SessionResource {
	return SessionResource{Name: name, CellName: cellName, Project: project, DisplayLabel: displayLabel, WorkingDirectory: workingDirectory, Windows: append([]SessionWindow(nil), windows...)}
}
