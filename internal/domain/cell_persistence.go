package domain

import "fmt"

type StoredCell struct {
	Version            uint64       `json:"version"`
	ID                 string       `json:"id"`
	Issue              string       `json:"issue"`
	Project            string       `json:"project"`
	Note               string       `json:"note"`
	Template           string       `json:"template"`
	Sources            Sources      `json:"sources"`
	Containers         Containers   `json:"containers"`
	Session            Session      `json:"session"`
	NotificationDriver string       `json:"notificationDriver"`
	Creation           CellCreation `json:"creation"`
	Status             string       `json:"status"`
	Done               bool         `json:"done"`
}

func NewStoredCell(version uint64, id string, issue string, project string, note string, templateName string, sources Sources, containers Containers, session Session, notificationDriver string, creation CellCreation, status string, done bool) StoredCell {
	return StoredCell{Version: version, ID: id, Issue: issue, Project: project, Note: note, Template: templateName, Sources: sources, Containers: containers, Session: session, NotificationDriver: notificationDriver, Creation: creation, Status: status, Done: done}
}

func RestoreCell(stored StoredCell) (Cell, error) {
	version, err := NewCellVersion(stored.Version)
	if err != nil {
		return Cell{}, err
	}
	status, err := NewCellStatus(stored.Status)
	if err != nil {
		return Cell{}, err
	}
	creationStatus, err := NewCreationStatus(string(stored.Creation.Status))
	if err != nil {
		return Cell{}, err
	}
	stored.Creation.Status = creationStatus
	for _, stage := range stored.Creation.CompletedStages {
		if _, err := NewCreationStage(string(stage)); err != nil {
			return Cell{}, err
		}
	}
	if stored.Creation.FailedStage != "" {
		if _, err := NewCreationStage(string(stored.Creation.FailedStage)); err != nil {
			return Cell{}, err
		}
	}
	sourceDriver, err := NewSourceDriverType(string(stored.Sources.Driver))
	if err != nil {
		return Cell{}, err
	}
	containerDriver := stored.Containers.Driver
	switch stored.Containers.Driver {
	case None, Docker:
	default:
		return Cell{}, fmt.Errorf("invalid container driver type %q", stored.Containers.Driver)
	}
	sessionDriver, err := NewSessionDriverType(string(stored.Session.Driver))
	if err != nil {
		return Cell{}, err
	}
	sources := make([]Source, 0, len(stored.Sources.Items))
	for _, source := range stored.Sources.Items {
		validated, err := NewSource(source.Path, source.Base, source.Branch)
		if err != nil {
			return Cell{}, err
		}
		sources = append(sources, validated)
	}
	containers := make([]Container, 0, len(stored.Containers.Items))
	for _, container := range stored.Containers.Items {
		validated, err := NewContainer(container.Network, container.SourceContainer, container.Mode)
		if err != nil {
			return Cell{}, err
		}
		containers = append(containers, validated)
	}
	windows := make([]SessionWindow, 0, len(stored.Session.Windows))
	for _, window := range stored.Session.Windows {
		validated, err := NewSessionWindow(window.Name, window.Command)
		if err != nil {
			return Cell{}, err
		}
		windows = append(windows, validated)
	}
	notificationDriver, err := NewNotificationDriverType(stored.NotificationDriver)
	if err != nil {
		return Cell{}, err
	}
	cell, err := NewCell(stored.ID, stored.Issue, stored.Project, stored.Template, NewSources(sourceDriver, sources), NewContainers(containerDriver, containers), NewSession(sessionDriver, windows), notificationDriver)
	if err != nil {
		return Cell{}, err
	}
	cell.Version, cell.Note, cell.Creation, cell.Status, cell.Done = version, stored.Note, stored.Creation, status, stored.Done
	return cell, nil
}
