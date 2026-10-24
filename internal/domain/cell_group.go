package domain

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
)

// CellGroup is the aggregate root that holds the shared identity, configuration,
// and creation state of a set of runtime Cells. Membership is recorded only by
// CommanderCell, TargetCell, and DependencyCell through their CellGroup ID.
type CellGroup struct {
	ID                 string
	Note               string
	Issue              string
	Project            string
	Template           string
	NotificationDriver NotificationDriverType
	SourceDriver       SourceDriverType
	ContainerDriver    ContainerDriverType
	Creation           CellCreation
}

func NewCellGroup(id, issue, project, template string, source SourceDriverType, container ContainerDriverType, notification NotificationDriverType) (CellGroup, error) {
	if id == "" || issue == "" || template == "" {
		return CellGroup{}, fmt.Errorf("cell group id, issue, and template are required")
	}
	if _, err := NewSourceDriverType(string(source)); err != nil {
		return CellGroup{}, err
	}
	if container != None && container != Docker {
		return CellGroup{}, fmt.Errorf("invalid container driver type %q", container)
	}
	validatedNotification, err := NewNotificationDriverType(string(notification))
	if err != nil {
		return CellGroup{}, err
	}
	return CellGroup{
		ID: id, Issue: issue, Project: project, Template: template,
		SourceDriver: source, ContainerDriver: container, NotificationDriver: validatedNotification,
		Creation: NewCellCreation(),
	}, nil
}

func (c *CellGroup) SetNote(note string) error {
	normalized, err := NormalizeCellNote(note)
	if err != nil {
		return err
	}
	c.Note = normalized
	return nil
}

func NormalizeCellNote(note string) (string, error) {
	normalized := strings.Join(strings.FieldsFunc(note, unicode.IsSpace), " ")
	length := len([]rune(normalized))
	if length == 0 || length > 20 {
		return "", fmt.Errorf("cell note must be between 1 and 20 characters after whitespace normalization")
	}
	return normalized, nil
}

func (c *CellGroup) BeginCreation() {
	creation := NewCellCreation()
	creation.Status = CreationCreating
	c.Creation = creation
}

func (c *CellGroup) FailCreation(stage CreationStage, err error) {
	c.Creation.Status = CreationFailed
	c.Creation.FailedStage = stage
	c.Creation.LastError = ""
	if err != nil {
		c.Creation.LastError = err.Error()
	}
}

func (c *CellGroup) FinishCreation() {
	c.Creation.Status = CreationReady
	c.Creation.FailedStage = ""
	c.Creation.LastError = ""
}

func (c CellGroup) Name() CellName {
	return NewCellName(c.Issue)
}

// Matches reports whether identifier names this CellGroup by its ID, issue, or Cell name.
func (c CellGroup) Matches(identifier string) bool {
	return c.ID == identifier || c.Issue == identifier || c.Name().Value == identifier
}

func (c CellGroup) ResourcePrefix() string {
	return fmt.Sprintf("paracell-%s-%s", SafeResourceName(c.Project, "project"), c.Name().Value)
}

func (c CellGroup) ResourceDrivers(workspaceDriver WorkspaceDriverType) CellDrivers {
	return NewCellDrivers(c.SourceDriver, c.ContainerDriver, workspaceDriver, c.NotificationDriver)
}

func (c CellGroup) WorkspaceName() string {
	return SafeResourceName(c.Project, "project") + "-" + c.Name().Value
}

// WorkspaceResource builds the workspace resource of this group's CommanderCell.
// Its display label is the note, or the Cell name when the note is empty.
func (c CellGroup) WorkspaceResource(workspace Workspace) WorkspaceResource {
	workingDirectory := filepath.Join(".paracell", "cells", c.Name().Value)
	label := c.Note
	if label == "" {
		label = c.Name().Value
	}
	return NewWorkspaceResource(c.WorkspaceName(), c.Name().Value, c.Project, label, workingDirectory, workspace.Windows)
}

func (c CellGroup) SourceWorktreePath(targetName string) string {
	return filepath.Join(".paracell", "cells", c.Name().Value, SafeResourceName(targetName, "target"), "source")
}
