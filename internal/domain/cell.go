package domain

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
)

type Cell struct {
	Version            CellVersion
	ID                 string
	Issue              string
	Project            string
	Note               string
	Template           string
	Sources            Sources
	Containers         Containers
	Session            Session
	NotificationDriver NotificationDriverType
	Creation           CellCreation
	Status             CellStatus
	Done               bool
}

func NewCell(id string, issue string, project string, templateName string, sources Sources, containers Containers, session Session, notificationDriver NotificationDriverType) (Cell, error) {
	if id == "" {
		return Cell{}, fmt.Errorf("cell id is required")
	}
	if issue == "" {
		return Cell{}, fmt.Errorf("issue is required")
	}
	if templateName == "" {
		return Cell{}, fmt.Errorf("template name is required")
	}
	version, err := NewCellVersion(1)
	if err != nil {
		return Cell{}, err
	}
	return Cell{
		Version: version, ID: id, Issue: issue, Project: project,
		Template: templateName, Sources: sources, Containers: containers, Session: session, NotificationDriver: notificationDriver,
		Creation: NewCellCreation(), Status: Ready,
	}, nil
}

func (c Cell) Name() CellName {
	return NewCellName(c.Issue)
}

func (c Cell) ResourcePrefix() string {
	return fmt.Sprintf("paracell-%s-%s", SafeResourceName(c.Project, "project"), c.Name().Value)
}

func (c Cell) DisplayLabel() string {
	if c.Note != "" {
		return c.Note
	}
	return c.Name().Value
}

func (c Cell) Summary() CellSummary {
	return NewCellSummary(c.Version, c.ID, c.Issue, c.Name().Value, c.DisplayLabel(), c.Template, c.CreationStatus(), c.Status, c.Done, c.Creation.FailedStage, c.Creation.LastError)
}

func (c Cell) ResourceDrivers() CellDrivers {
	return NewCellDrivers(c.Sources.Driver, c.Containers.Driver, c.Session.Driver, c.NotificationDriver)
}

func NormalizeCellNote(note string) (string, error) {
	normalized := strings.Join(strings.FieldsFunc(note, unicode.IsSpace), " ")
	length := len([]rune(normalized))
	if length == 0 || length > 20 {
		return "", fmt.Errorf("cell note must be between 1 and 20 characters after whitespace normalization")
	}
	return normalized, nil
}

func (c *Cell) SetNote(note string) error {
	normalized, err := NormalizeCellNote(note)
	if err != nil {
		return err
	}
	c.Note = normalized
	return nil
}

func (c *Cell) MarkDone() error {
	if c.Done {
		return fmt.Errorf("cell is already done")
	}
	c.Done = true
	return nil
}

func (c *Cell) ToggleDone() {
	c.Done = !c.Done
}

func (c *Cell) SetStatus(status CellStatus) error {
	validated, err := NewCellStatus(string(status))
	if err != nil {
		return err
	}
	c.Status = validated
	return nil
}

func (c Cell) EnsureCanBeCleaned() error {
	if !c.Done {
		return fmt.Errorf("完了済みではないので消せない")
	}
	return nil
}

func ResolveCell(cells []Cell, identifier string) (Cell, bool) {
	for _, cell := range cells {
		if cell.Matches(identifier) {
			return cell, true
		}
	}
	return Cell{}, false
}

func (c Cell) Matches(identifier string) bool {
	return c.ID == identifier || c.Issue == identifier || c.Name().Value == identifier
}

func EnsureCellUnique(existing []Cell, issue string, name CellName) error {
	for _, cell := range existing {
		if cell.Issue == issue {
			return fmt.Errorf("cell issue %q already exists", issue)
		}
		if cell.Name() == name {
			return fmt.Errorf("cell name %q already exists", name.Value)
		}
	}
	return nil
}

func (c *Cell) AdvanceVersion() error {
	version, err := c.Version.Add()
	if err != nil {
		return err
	}
	c.Version = version
	return nil
}

func (c Cell) Stored() StoredCell {
	return NewStoredCell(uint64(c.Version), c.ID, c.Issue, c.Project, c.Note, c.Template, c.Sources, c.Containers, c.Session, string(c.NotificationDriver), c.Creation, string(c.Status), c.Done)
}

func (c Cell) Clone() Cell {
	c.Sources.Items = append([]Source(nil), c.Sources.Items...)
	c.Containers.Items = append([]Container(nil), c.Containers.Items...)
	for i := range c.Containers.Items {
		c.Containers.Items[i].Network = append([]string(nil), c.Containers.Items[i].Network...)
	}
	c.Session.Windows = append([]SessionWindow(nil), c.Session.Windows...)
	return c
}

func (c Cell) SourceWorktreePath(source Source) string {
	path := filepath.Join(".paracell", "cells", c.Name().Value, "source")
	if source.Path != "." {
		path = filepath.Join(path, source.Path)
	}
	return path
}

func (c Cell) ContainerNetworkName() string {
	return c.ResourcePrefix()
}

func (c Cell) ContainerResourceName(container Container) string {
	if container.Mode == Dependency {
		return container.SourceContainer
	}
	return c.ResourcePrefix() + "-" + SafeResourceName(container.SourceContainer, "container")
}

func (c Cell) UsesDependency() bool {
	for _, container := range c.Containers.Items {
		if container.Mode == Dependency {
			return true
		}
	}
	return false
}

func (c *Cell) RecordContainerNetworks(networks map[string][]string) {
	for index := range c.Containers.Items {
		c.Containers.Items[index].Network = append([]string(nil), networks[c.Containers.Items[index].SourceContainer]...)
	}
}

func (c *Cell) PlanSources(sources Sources) {
	c.Sources = sources
}

func (c *Cell) PlanContainers(containers Containers) {
	c.Containers = containers
}

func (c *Cell) PlanSession(session Session) {
	c.Session = session
}

func (c Cell) SessionName() string {
	return SafeResourceName(c.Project, "project") + "-" + c.Name().Value
}

func (c *Cell) BeginCreation() {
	creation := NewCellCreation()
	creation.Status = CreationCreating
	c.Creation = creation
}

func (c *Cell) FailCreation(stage CreationStage, err error) {
	c.Creation.Status = CreationFailed
	c.Creation.FailedStage = stage
	c.Creation.LastError = ""
	if err != nil {
		c.Creation.LastError = err.Error()
	}
}

func (c *Cell) FinishCreation() {
	c.Creation.Status = CreationReady
	c.Creation.FailedStage = ""
	c.Creation.LastError = ""
}

func (c Cell) CreationStatus() CreationStatus {
	return c.Creation.Status
}

func (c Cell) sourceResources() []SourceResource {
	resources := make([]SourceResource, 0, len(c.Sources.Items))
	for _, source := range c.Sources.Items {
		resources = append(resources, NewSourceResource(source.Path, c.SourceWorktreePath(source), source.Base, source.Branch))
	}
	return resources
}

func (c Cell) containerResources(templates []ContainerTemplate) ContainerResources {
	bySourceContainer := make(map[string]ContainerTemplate, len(templates))
	for _, template := range templates {
		bySourceContainer[template.Name] = template
	}
	items := make([]ContainerResource, 0, len(c.Containers.Items))
	for _, container := range c.Containers.Items {
		template := bySourceContainer[container.SourceContainer]
		items = append(items, NewContainerResource(
			c.ContainerResourceName(container), container.Network,
			container.SourceContainer, container.Mode, template.Environments, template.Mounts,
		))
	}
	sourcePath := ""
	if len(c.Sources.Items) > 0 {
		sourcePath = c.SourceWorktreePath(c.Sources.Items[0])
		if sourcePath != "" {
			sourcePath = filepath.Clean(sourcePath)
		}
	}
	return NewContainerResources(c.Name().Value, c.Project, c.ContainerNetworkName(), sourcePath, items)
}

func (c Cell) sessionResource() SessionResource {
	workingDirectory := ""
	if len(c.Sources.Items) > 0 {
		workingDirectory = c.SourceWorktreePath(c.Sources.Items[0])
	}
	return NewSessionResource(c.SessionName(), c.Name().Value, c.Project, c.DisplayLabel(), workingDirectory, c.Session.Windows)
}
