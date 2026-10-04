package domain

import "fmt"

type WorkspaceWindow struct {
	Name    string
	Command string
}

func NewWorkspaceWindow(name string, command string) (WorkspaceWindow, error) {
	if name == "" {
		return WorkspaceWindow{}, fmt.Errorf("workspace window name is required")
	}
	return WorkspaceWindow{Name: name, Command: command}, nil
}
