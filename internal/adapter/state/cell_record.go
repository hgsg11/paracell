package state

import "github.com/hgsg11/paracell/internal/domain"

// cellRecord is the JSON stored in the record column of one cells row: one
// CellGroup and the Cells that belong to it. The adapter owns this shape so that
// domain types are never serialized directly. The row version lives only in the
// version column.
type cellRecord struct {
	CellGroup    *cellGroupRecord   `json:"cellGroup"`
	Commander    commanderRecord    `json:"commander"`
	Targets      []targetRecord     `json:"targets"`
	Dependencies []dependencyRecord `json:"dependencies"`
}

type cellGroupRecord struct {
	ID                 string         `json:"id"`
	Note               string         `json:"note"`
	Issue              string         `json:"issue"`
	Project            string         `json:"project"`
	Template           string         `json:"template"`
	NotificationDriver string         `json:"notificationDriver"`
	SourceDriver       string         `json:"sourceDriver"`
	ContainerDriver    string         `json:"containerDriver"`
	Creation           creationRecord `json:"creation"`
}

type creationRecord struct {
	Status      string `json:"status"`
	FailedStage string `json:"failedStage"`
	LastError   string `json:"lastError"`
}

type commanderRecord struct {
	ID          string          `json:"id"`
	CellGroupID string          `json:"cellGroupId"`
	Workspace   workspaceRecord `json:"workspace"`
	Status      string          `json:"status"`
	Done        bool            `json:"done"`
}

type workspaceRecord struct {
	Driver  string         `json:"driver"`
	Windows []windowRecord `json:"windows"`
}

type windowRecord struct {
	Name    string `json:"name"`
	Command string `json:"command"`
}

type targetRecord struct {
	ID          string            `json:"id"`
	CellGroupID string            `json:"cellGroupId"`
	Name        string            `json:"name"`
	Source      *sourceRecord     `json:"source"`
	Containers  []containerRecord `json:"containers"`
}

type sourceRecord struct {
	Path   string `json:"path"`
	Base   string `json:"base"`
	Branch string `json:"branch"`
}

type containerRecord struct {
	Network         []string `json:"network"`
	SourceContainer string   `json:"sourceContainer"`
	Mode            string   `json:"mode"`
}

type dependencyRecord struct {
	ID          string          `json:"id"`
	CellGroupID string          `json:"cellGroupId"`
	Name        string          `json:"name"`
	Container   containerRecord `json:"container"`
}

func newCellRecord(group domain.CellGroup, commander domain.CommanderCell, targets []domain.TargetCell, dependencies []domain.DependencyCell) cellRecord {
	record := cellRecord{
		CellGroup: &cellGroupRecord{
			ID: group.ID, Note: group.Note, Issue: group.Issue, Project: group.Project, Template: group.Template,
			NotificationDriver: string(group.NotificationDriver), SourceDriver: string(group.SourceDriver), ContainerDriver: string(group.ContainerDriver),
			Creation: creationRecord{Status: string(group.Creation.Status), FailedStage: string(group.Creation.FailedStage), LastError: group.Creation.LastError},
		},
		Commander: commanderRecord{
			ID: commander.ID, CellGroupID: commander.CellGroupID,
			Workspace: workspaceRecord{Driver: string(commander.Workspace.Driver)},
			Status:    string(commander.Status), Done: commander.Done,
		},
	}
	for _, window := range commander.Workspace.Windows {
		record.Commander.Workspace.Windows = append(record.Commander.Workspace.Windows, windowRecord{Name: window.Name, Command: window.Command})
	}
	for _, target := range targets {
		item := targetRecord{ID: target.ID, CellGroupID: target.CellGroupID, Name: target.Name}
		if target.Source != nil {
			item.Source = &sourceRecord{Path: target.Source.Path, Base: target.Source.Base, Branch: target.Source.Branch}
		}
		for _, container := range target.Containers {
			item.Containers = append(item.Containers, newContainerRecord(*container))
		}
		record.Targets = append(record.Targets, item)
	}
	for _, dependency := range dependencies {
		record.Dependencies = append(record.Dependencies, dependencyRecord{
			ID: dependency.ID, CellGroupID: dependency.CellGroupID, Name: dependency.Name, Container: newContainerRecord(dependency.Container),
		})
	}
	return record
}

func newContainerRecord(container domain.Container) containerRecord {
	record := containerRecord{SourceContainer: container.SourceContainer, Mode: string(container.Mode)}
	if len(container.Network) > 0 {
		record.Network = append([]string(nil), container.Network...)
	}
	return record
}

// restore rebuilds the domain models of one row. The factories validate the raw
// values; Target and Dependency Cells keep their existing restore functions.
func (r cellRecord) restore(version uint64) (domain.CellGroup, domain.CommanderCell, []domain.TargetCell, []domain.DependencyCell, error) {
	raw := r.CellGroup
	group, err := domain.CellGroupFactory{}.Restore(raw.ID, raw.Issue, raw.Project, raw.Template, raw.Note, raw.SourceDriver, raw.ContainerDriver, raw.NotificationDriver, raw.Creation.Status, raw.Creation.FailedStage, raw.Creation.LastError)
	if err != nil {
		return domain.CellGroup{}, domain.CommanderCell{}, nil, nil, err
	}
	windows := make([]domain.RestoredWorkspaceWindow, 0, len(r.Commander.Workspace.Windows))
	for _, window := range r.Commander.Workspace.Windows {
		windows = append(windows, domain.RestoredWorkspaceWindow{Name: window.Name, Command: window.Command})
	}
	commander, err := domain.CommanderCellFactory{}.Restore(r.Commander.ID, r.Commander.CellGroupID, r.Commander.Workspace.Driver, windows, r.Commander.Status, r.Commander.Done, version)
	if err != nil {
		return domain.CellGroup{}, domain.CommanderCell{}, nil, nil, err
	}
	targets := make([]domain.TargetCell, 0, len(r.Targets))
	for _, stored := range r.Targets {
		target, err := domain.RestoreTargetCell(stored.toDomain())
		if err != nil {
			return domain.CellGroup{}, domain.CommanderCell{}, nil, nil, err
		}
		targets = append(targets, target)
	}
	dependencies := make([]domain.DependencyCell, 0, len(r.Dependencies))
	for _, stored := range r.Dependencies {
		dependency, err := domain.RestoreDependencyCell(domain.DependencyCell{ID: stored.ID, CellGroupID: stored.CellGroupID, Name: stored.Name, Container: stored.Container.toDomain()})
		if err != nil {
			return domain.CellGroup{}, domain.CommanderCell{}, nil, nil, err
		}
		dependencies = append(dependencies, dependency)
	}
	return group, commander, targets, dependencies, nil
}

func (r targetRecord) toDomain() domain.TargetCell {
	target := domain.TargetCell{ID: r.ID, CellGroupID: r.CellGroupID, Name: r.Name}
	if r.Source != nil {
		target.Source = &domain.Source{Path: r.Source.Path, Base: r.Source.Base, Branch: r.Source.Branch}
	}
	for _, container := range r.Containers {
		value := container.toDomain()
		target.Containers = append(target.Containers, &value)
	}
	return target
}

func (r containerRecord) toDomain() domain.Container {
	return domain.Container{Network: append([]string(nil), r.Network...), SourceContainer: r.SourceContainer, Mode: domain.Mode(r.Mode)}
}
