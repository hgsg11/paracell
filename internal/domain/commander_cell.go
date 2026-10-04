package domain

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
)

// CommanderCell is an independent runtime Cell with references to related Cells.
type CommanderCell struct {
	ID                 string
	Issue              string
	Project            string
	Note               string
	Template           string
	Version            CellVersion
	Workspace          Workspace
	Targets            []string
	Dependencies       []string
	Status             CellStatus
	Creation           CellCreation
	Done               bool
	NotificationDriver NotificationDriverType
	SourceDriver       SourceDriverType
	ContainerDriver    ContainerDriverType
}

func NewCommanderCell(id, issue, project, templateName string, workspace Workspace, targets, dependencies []string, sourceDriver SourceDriverType, containerDriver ContainerDriverType, notificationDriver NotificationDriverType) (CommanderCell, error) {
	if id == "" || issue == "" || templateName == "" {
		return CommanderCell{}, fmt.Errorf("commander cell id, issue, and template are required")
	}
	version, err := NewCellVersion(1)
	if err != nil {
		return CommanderCell{}, err
	}
	if _, err := NewWorkspaceDriverType(string(workspace.Driver)); err != nil {
		return CommanderCell{}, err
	}
	validatedNotificationDriver, err := NewNotificationDriverType(string(notificationDriver))
	if err != nil {
		return CommanderCell{}, err
	}
	for _, reference := range append(append([]string(nil), targets...), dependencies...) {
		if reference == "" {
			return CommanderCell{}, fmt.Errorf("commander cell %q has an empty Cell reference", NewCellName(issue).Value)
		}
	}
	if containerDriver != None && containerDriver != Docker {
		return CommanderCell{}, fmt.Errorf("invalid container driver type %q", containerDriver)
	}
	validatedSourceDriver, err := NewSourceDriverType(string(sourceDriver))
	if err != nil {
		return CommanderCell{}, err
	}
	return CommanderCell{
		ID: id, Issue: issue, Project: project, Template: templateName,
		Version: version, Workspace: NewWorkspace(workspace.Driver, workspace.Windows),
		Targets: append([]string(nil), targets...), Dependencies: append([]string(nil), dependencies...), Status: Ready,
		Creation: NewCellCreation(), NotificationDriver: validatedNotificationDriver, SourceDriver: validatedSourceDriver, ContainerDriver: containerDriver,
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
	if c.Note != "" {
		return c.Note
	}
	return c.Name().Value
}

func (c CommanderCell) ListLabels() (string, string) {
	return c.DisplayLabel(), c.Template
}

func (c CommanderCell) HasStatus(status CellStatus) bool {
	return c.Status == status
}

func (c *CommanderCell) SetNote(note string) error {
	normalized, err := NormalizeCellNote(note)
	if err != nil {
		return err
	}
	c.Note = normalized
	return nil
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
	return c.ID == identifier || c.Issue == identifier || c.Name().Value == identifier
}

func NormalizeCellNote(note string) (string, error) {
	normalized := strings.Join(strings.FieldsFunc(note, unicode.IsSpace), " ")
	length := len([]rune(normalized))
	if length == 0 || length > 20 {
		return "", fmt.Errorf("cell note must be between 1 and 20 characters after whitespace normalization")
	}
	return normalized, nil
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
		if cell.Issue == issue {
			return fmt.Errorf("commander cell issue %q already exists", issue)
		}
		if cell.Name() == name {
			return fmt.Errorf("commander cell name %q already exists", name.Value)
		}
	}
	return nil
}

func (c CommanderCell) Name() CellName {
	return NewCellName(c.Issue)
}

func (c CommanderCell) ResourcePrefix() string {
	return fmt.Sprintf("paracell-%s-%s", SafeResourceName(c.Project, "project"), c.Name().Value)
}

func (c CommanderCell) ResourceDrivers() CellDrivers {
	return NewCellDrivers(c.SourceDriver, c.ContainerDriver, c.Workspace.Driver, c.NotificationDriver)
}

func (c CommanderCell) WorkspaceName() string {
	return SafeResourceName(c.Project, "project") + "-" + c.Name().Value
}

func (c CommanderCell) WorkspaceResource() WorkspaceResource {
	workingDirectory := filepath.Join(".paracell", "cells", c.Name().Value)
	windows := make([]WorkspaceWindow, len(c.Workspace.Windows))
	copy(windows, c.Workspace.Windows)
	return NewWorkspaceResource(c.WorkspaceName(), c.Name().Value, c.Project, c.DisplayLabel(), workingDirectory, windows)
}

func (c CommanderCell) SourceWorktreePath(targetName string) string {
	return filepath.Join(".paracell", "cells", c.Name().Value, SafeResourceName(targetName, "target"), "source")
}

func (c CommanderCell) CreationStatus() CreationStatus {
	return c.Creation.Status
}

func (c CommanderCell) CreationFailure() (CreationStage, string) {
	return c.Creation.FailedStage, c.Creation.LastError
}

func (c *CommanderCell) BeginCreation() {
	creation := NewCellCreation()
	creation.Status = CreationCreating
	c.Creation = creation
}

func (c *CommanderCell) FailCreation(stage CreationStage, err error) {
	c.Creation.Status = CreationFailed
	c.Creation.FailedStage = stage
	c.Creation.LastError = ""
	if err != nil {
		c.Creation.LastError = err.Error()
	}
}

func (c *CommanderCell) FinishCreation() {
	c.Creation.Status = CreationReady
	c.Creation.FailedStage = ""
	c.Creation.LastError = ""
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
	return NewStoredCommanderCell(uint64(c.Version), c.ID, c.Issue, c.Project, c.Note, c.Template, c.Workspace, c.Targets, c.Dependencies, c.SourceDriver, c.ContainerDriver, c.NotificationDriver, c.Creation, string(c.Status), c.Done)
}

func (c CommanderCell) Clone() CommanderCell {
	c.Workspace.Windows = append([]WorkspaceWindow(nil), c.Workspace.Windows...)
	c.Targets = append([]string(nil), c.Targets...)
	c.Dependencies = append([]string(nil), c.Dependencies...)
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

func (c CommanderCell) SourceResources(targets []TargetCell) []SourceResource {
	resources := make([]SourceResource, 0, len(targets))
	for _, target := range targets {
		if target.Source == nil {
			continue
		}
		path := c.SourceWorktreePath(target.Name)
		if target.Source.Path != "." {
			path = filepath.Join(path, target.Source.Path)
		}
		resources = append(resources, NewSourceResource(target.Source.Path, path, target.Source.Base, target.Source.Branch))
	}
	return resources
}
