package domain

import "fmt"

type StoredCommanderCell struct {
	Version   uint64       `json:"version"`
	ID        string       `json:"id"`
	CellGroup *CellGroup   `json:"cellGroup"`
	Workspace Workspace    `json:"workspace"`
	Creation  CellCreation `json:"creation"`
	Status    string       `json:"status"`
	Done      bool         `json:"done"`
}

func NewStoredCommanderCell(version uint64, id string, group *CellGroup, workspace Workspace, creation CellCreation, status string, done bool) StoredCommanderCell {
	copy := *group
	return StoredCommanderCell{Version: version, ID: id, CellGroup: &copy, Workspace: NewWorkspace(workspace.Driver, workspace.Windows), Creation: creation, Status: status, Done: done}
}

func RestoreCommanderCell(stored StoredCommanderCell) (CommanderCell, error) {
	version, err := NewCellVersion(stored.Version)
	if err != nil {
		return CommanderCell{}, err
	}
	status, err := NewCellStatus(stored.Status)
	if err != nil {
		return CommanderCell{}, err
	}
	creationStatus, err := NewCreationStatus(string(stored.Creation.Status))
	if err != nil {
		return CommanderCell{}, err
	}
	stored.Creation.Status = creationStatus
	if stored.Creation.FailedStage != "" {
		if _, err := NewCreationStage(string(stored.Creation.FailedStage)); err != nil {
			return CommanderCell{}, err
		}
	}
	workspaceDriver, err := NewWorkspaceDriverType(string(stored.Workspace.Driver))
	if err != nil {
		return CommanderCell{}, err
	}
	windows := make([]WorkspaceWindow, 0, len(stored.Workspace.Windows))
	for _, window := range stored.Workspace.Windows {
		validated, err := NewWorkspaceWindow(window.Name, window.Command)
		if err != nil {
			return CommanderCell{}, err
		}
		windows = append(windows, validated)
	}
	workspace := NewWorkspace(workspaceDriver, windows)
	if stored.CellGroup == nil {
		return CommanderCell{}, fmt.Errorf("CellGroup is required")
	}
	raw := stored.CellGroup
	group, err := NewCellGroup(raw.ID, raw.Issue, raw.Project, raw.Template, raw.SourceDriver, raw.ContainerDriver, raw.NotificationDriver)
	if err != nil {
		return CommanderCell{}, err
	}
	if raw.Note != "" {
		if err := group.SetNote(raw.Note); err != nil {
			return CommanderCell{}, err
		}
	}
	cell, err := NewCommanderCell(stored.ID, &group, workspace)
	if err != nil {
		return CommanderCell{}, err
	}
	cell.Version, cell.Creation, cell.Status, cell.Done = version, stored.Creation, status, stored.Done
	return cell, nil
}
