package domain

import "fmt"

// RestoredWorkspaceWindow is one persisted workspace window as raw strings.
type RestoredWorkspaceWindow struct {
	Name    string
	Command string
}

// CommanderCellFactory rebuilds persisted CommanderCells from raw values.
type CommanderCellFactory struct{}

// Restore validates the raw persisted values and rebuilds a CommanderCell. It is
// the only place that sets CommanderCell.Version.
func (CommanderCellFactory) Restore(id, cellGroupID, workspaceDriver string, windows []RestoredWorkspaceWindow, status string, done bool, version uint64) (CommanderCell, error) {
	driver, err := NewWorkspaceDriverType(workspaceDriver)
	if err != nil {
		return CommanderCell{}, err
	}
	validatedWindows := make([]WorkspaceWindow, 0, len(windows))
	for _, window := range windows {
		validated, err := NewWorkspaceWindow(window.Name, window.Command)
		if err != nil {
			return CommanderCell{}, err
		}
		validatedWindows = append(validatedWindows, validated)
	}
	validatedStatus, err := NewCellStatus(status)
	if err != nil {
		return CommanderCell{}, err
	}
	validatedVersion, err := NewCellVersion(version)
	if err != nil {
		return CommanderCell{}, fmt.Errorf("restore CommanderCell %q: %w", id, err)
	}
	cell, err := NewCommanderCell(id, cellGroupID, NewWorkspace(driver, validatedWindows))
	if err != nil {
		return CommanderCell{}, err
	}
	cell.Status, cell.Done, cell.Version = validatedStatus, done, validatedVersion
	return cell, nil
}
