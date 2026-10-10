package domain

import "fmt"

type StoredCommanderCell struct {
	Version     uint64    `json:"version"`
	ID          string    `json:"id"`
	CellGroupID string    `json:"cellGroupId"`
	Workspace   Workspace `json:"workspace"`
	Status      string    `json:"status"`
	Done        bool      `json:"done"`
}

func NewStoredCommanderCell(version uint64, id, cellGroupID string, workspace Workspace, status string, done bool) StoredCommanderCell {
	return StoredCommanderCell{Version: version, ID: id, CellGroupID: cellGroupID, Workspace: NewWorkspace(workspace.Driver, workspace.Windows), Status: status, Done: done}
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
	cell, err := NewCommanderCell(stored.ID, stored.CellGroupID, workspace)
	if err != nil {
		return CommanderCell{}, err
	}
	if stored.CellGroupID == "" {
		return CommanderCell{}, fmt.Errorf("CellGroup ID is required")
	}
	cell.Version, cell.Status, cell.Done = version, status, stored.Done
	return cell, nil
}
