package domain

import (
	"fmt"
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
