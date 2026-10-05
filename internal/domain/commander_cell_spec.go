package domain

import "fmt"

// CommanderCellSpec defines only the workspace-bearing CommanderCell input.
type CommanderCellSpec struct {
	Name      string
	Workspace WorkspaceTemplate
}

func NewCommanderCellSpec(name string, workspace WorkspaceTemplate) (CommanderCellSpec, error) {
	if name == "" {
		return CommanderCellSpec{}, fmt.Errorf("commander cell name is required")
	}
	return CommanderCellSpec{Name: name, Workspace: NewWorkspaceTemplate(workspace.Windows)}, nil
}

func cloneCommanderCellSpec(spec CommanderCellSpec) (CommanderCellSpec, error) {
	return NewCommanderCellSpec(spec.Name, spec.Workspace)
}
