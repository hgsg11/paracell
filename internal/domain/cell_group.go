package domain

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
)

// CellGroup holds the shared identity and configuration of a set of runtime Cells.
// Membership is recorded only by TargetCell and DependencyCell.
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
	return CellGroup{ID: id, Issue: issue, Project: project, Template: template, SourceDriver: source, ContainerDriver: container, NotificationDriver: validatedNotification, Creation: NewCellCreation()}, nil
}

func RestoreCellGroup(stored CellGroup) (CellGroup, error) {
	group, err := NewCellGroup(stored.ID, stored.Issue, stored.Project, stored.Template, stored.SourceDriver, stored.ContainerDriver, stored.NotificationDriver)
	if err != nil {
		return CellGroup{}, err
	}
	if stored.Note != "" {
		if err := group.SetNote(stored.Note); err != nil {
			return CellGroup{}, err
		}
	}
	if stored.Creation.Status == "" {
		stored.Creation = NewCellCreation()
	} else {
		status, err := NewCreationStatus(string(stored.Creation.Status))
		if err != nil {
			return CellGroup{}, err
		}
		stored.Creation.Status = status
	}
	if stored.Creation.FailedStage != "" {
		stage, err := NewCreationStage(string(stored.Creation.FailedStage))
		if err != nil {
			return CellGroup{}, err
		}
		stored.Creation.FailedStage = stage
	}
	group.Creation = stored.Creation
	return group, nil
}

func (c *CellGroup) BeginCreation() {
	c.Creation = CellCreation{Status: CreationCreating}
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
	c.Creation = NewCellCreation()
}

func (c CellGroup) CreationStatus() CreationStatus { return c.Creation.Status }

func (c CellGroup) CreationFailure() (CreationStage, string) {
	return c.Creation.FailedStage, c.Creation.LastError
}

func (c CellGroup) Name() CellName { return NewCellName(c.Issue) }

func (c CellGroup) DisplayLabel() string {
	if c.Note != "" {
		return c.Note
	}
	return c.Name().Value
}

func (c CellGroup) ResourcePrefix() string {
	return fmt.Sprintf("paracell-%s-%s", SafeResourceName(c.Project, "project"), c.Name().Value)
}

func (c CellGroup) ResourceDrivers(workspace WorkspaceDriverType) CellDrivers {
	return NewCellDrivers(c.SourceDriver, c.ContainerDriver, workspace, c.NotificationDriver)
}

func (c CellGroup) WorkspaceName() string {
	return SafeResourceName(c.Project, "project") + "-" + c.Name().Value
}

func (c CellGroup) WorkspaceResource(workspace Workspace) WorkspaceResource {
	windows := make([]WorkspaceWindow, len(workspace.Windows))
	copy(windows, workspace.Windows)
	return NewWorkspaceResource(c.WorkspaceName(), c.Name().Value, c.Project, c.DisplayLabel(), filepath.Join(".paracell", "cells", c.Name().Value), windows)
}

func (c CellGroup) SourceWorktreePath(targetName string) string {
	return filepath.Join(".paracell", "cells", c.Name().Value, SafeResourceName(targetName, "target"), "source")
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
