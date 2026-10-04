package domain

type Workspace struct {
	Driver  WorkspaceDriverType
	Windows []WorkspaceWindow
}

func NewWorkspace(driver WorkspaceDriverType, windows []WorkspaceWindow) Workspace {
	return Workspace{Driver: driver, Windows: append([]WorkspaceWindow(nil), windows...)}
}
