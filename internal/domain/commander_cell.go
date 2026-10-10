package domain

import (
	"fmt"
)

// CommanderCell holds commands and execution state, referring to shared CellGroup information.
type CommanderCell struct {
	ID          string
	CellGroupID string
	Version     CellVersion
	Workspace   Workspace
	Status      CellStatus
	Done        bool
}

func NewCommanderCell(id, cellGroupID string, workspace Workspace) (CommanderCell, error) {
	if id == "" || cellGroupID == "" {
		return CommanderCell{}, fmt.Errorf("commander cell id and CellGroup ID are required")
	}
	version, err := NewCellVersion(1)
	if err != nil {
		return CommanderCell{}, err
	}
	return CommanderCell{
		ID: id, CellGroupID: cellGroupID, Version: version,
		Workspace: NewWorkspace(workspace.Driver, workspace.Windows),
		Status:    Ready,
	}, nil
}

func (c *CommanderCell) SetStatus(status CellStatus) error {
	validated, err := NewCellStatus(string(status))
	if err != nil {
		return err
	}
	c.Status = validated
	return nil
}

func (c CommanderCell) EnsureCanBeCleaned() error {
	if !c.Done {
		return fmt.Errorf("完了済みではないので消せない")
	}
	return nil
}

func (c CommanderCell) Matches(identifier string) bool {
	return c.ID == identifier || c.CellGroupID == identifier
}

func ResolveCommanderCell(cells []CommanderCell, identifier string) (CommanderCell, bool) {
	for _, cell := range cells {
		if cell.Matches(identifier) {
			return cell, true
		}
	}
	return CommanderCell{}, false
}

func (c *CommanderCell) AdvanceVersion() error {
	version, err := c.Version.Add()
	if err != nil {
		return err
	}
	c.Version = version
	return nil
}

func (c CommanderCell) Stored() StoredCommanderCell {
	return NewStoredCommanderCell(uint64(c.Version), c.ID, c.CellGroupID, c.Workspace, string(c.Status), c.Done)
}

func (c CommanderCell) Clone() CommanderCell {
	c.Workspace.Windows = append([]WorkspaceWindow(nil), c.Workspace.Windows...)
	return c
}

func (c *CommanderCell) ToggleDone() {
	c.Done = !c.Done
}
