package domain

import (
	"fmt"
	"path/filepath"
)

// CommanderCell holds commands and execution state, referring to shared CellGroup information.
type CommanderCell struct {
	ID        string
	CellGroup *CellGroup
	Version   CellVersion
	Workspace Workspace
	Status    CellStatus
	Done      bool
}

func NewCommanderCell(id string, group *CellGroup, workspace Workspace) (CommanderCell, error) {
	if id == "" || group == nil {
		return CommanderCell{}, fmt.Errorf("commander cell id and CellGroup are required")
	}
	version, err := NewCellVersion(1)
	if err != nil {
		return CommanderCell{}, err
	}
	return CommanderCell{
		ID: id, CellGroup: group, Version: version,
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

func (c CommanderCell) DisplayLabel() string {
	if c.CellGroup.Note != "" {
		return c.CellGroup.Note
	}
	return c.Name().Value
}

func (c CommanderCell) ListLabels() (string, string) {
	return c.DisplayLabel(), c.CellGroup.Template
}

func (c CommanderCell) HasStatus(status CellStatus) bool {
	return c.Status == status
}

func (c CommanderCell) EnsureCanBeCleaned() error {
	if !c.Done {
		return fmt.Errorf("完了済みではないので消せない")
	}
	return nil
}

func (c CommanderCell) SameIdentity(other CommanderCell) bool {
	return c.ID == other.ID
}

func (c CommanderCell) Matches(identifier string) bool {
	return c.ID == identifier || c.CellGroup.ID == identifier || c.CellGroup.Issue == identifier || c.Name().Value == identifier
}

func ResolveCommanderCell(cells []CommanderCell, identifier string) (CommanderCell, bool) {
	for _, cell := range cells {
		if cell.Matches(identifier) {
			return cell, true
		}
	}
	return CommanderCell{}, false
}

func EnsureCommanderCellUnique(cells []CommanderCell, issue string, name CellName) error {
	for _, cell := range cells {
		if cell.CellGroup.Issue == issue {
			return fmt.Errorf("commander cell issue %q already exists", issue)
		}
		if cell.Name() == name {
			return fmt.Errorf("commander cell name %q already exists", name.Value)
		}
	}
	return nil
}

func (c CommanderCell) Name() CellName {
	return NewCellName(c.CellGroup.Issue)
}

func (c CommanderCell) ResourcePrefix() string {
	return fmt.Sprintf("paracell-%s-%s", SafeResourceName(c.CellGroup.Project, "project"), c.Name().Value)
}

func (c CommanderCell) ResourceDrivers() CellDrivers {
	return NewCellDrivers(c.CellGroup.SourceDriver, c.CellGroup.ContainerDriver, c.Workspace.Driver, c.CellGroup.NotificationDriver)
}

func (c CommanderCell) WorkspaceName() string {
	return SafeResourceName(c.CellGroup.Project, "project") + "-" + c.Name().Value
}

func (c CommanderCell) WorkspaceResource() WorkspaceResource {
	workingDirectory := filepath.Join(".paracell", "cells", c.Name().Value)
	windows := make([]WorkspaceWindow, len(c.Workspace.Windows))
	copy(windows, c.Workspace.Windows)
	return NewWorkspaceResource(c.WorkspaceName(), c.Name().Value, c.CellGroup.Project, c.DisplayLabel(), workingDirectory, windows)
}

func (c CommanderCell) SourceWorktreePath(targetName string) string {
	return filepath.Join(".paracell", "cells", c.Name().Value, SafeResourceName(targetName, "target"), "source")
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
	return NewStoredCommanderCell(uint64(c.Version), c.ID, c.CellGroup, c.Workspace, c.CellGroup.Creation, string(c.Status), c.Done)
}

func (c CommanderCell) Clone() CommanderCell {
	c.Workspace.Windows = append([]WorkspaceWindow(nil), c.Workspace.Windows...)
	group := *c.CellGroup
	c.CellGroup = &group
	return c
}

func (c *CommanderCell) ToggleDone() {
	c.Done = !c.Done
}

func (c *CommanderCell) MarkDone() error {
	if c.Done {
		return fmt.Errorf("cell is already done")
	}
	c.Done = true
	return nil
}
