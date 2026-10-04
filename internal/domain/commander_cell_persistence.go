package domain

type StoredCommanderCell struct {
	Version            uint64                 `json:"version"`
	ID                 string                 `json:"id"`
	Issue              string                 `json:"issue"`
	Project            string                 `json:"project"`
	Note               string                 `json:"note"`
	Template           string                 `json:"template"`
	Workspace          Workspace              `json:"workspace"`
	Targets            []string               `json:"targets"`
	Dependencies       []string               `json:"dependencies"`
	SourceDriver       SourceDriverType       `json:"sourceDriver"`
	ContainerDriver    ContainerDriverType    `json:"containerDriver"`
	NotificationDriver NotificationDriverType `json:"notificationDriver"`
	Creation           CellCreation           `json:"creation"`
	Status             string                 `json:"status"`
	Done               bool                   `json:"done"`
}

func NewStoredCommanderCell(version uint64, id, issue, project, note, template string, workspace Workspace, targets, dependencies []string, sourceDriver SourceDriverType, containerDriver ContainerDriverType, notificationDriver NotificationDriverType, creation CellCreation, status string, done bool) StoredCommanderCell {
	return StoredCommanderCell{
		Version: version, ID: id, Issue: issue, Project: project, Note: note, Template: template,
		Workspace: NewWorkspace(workspace.Driver, workspace.Windows), Targets: append([]string(nil), targets...),
		Dependencies: append([]string(nil), dependencies...), SourceDriver: sourceDriver, ContainerDriver: containerDriver,
		NotificationDriver: notificationDriver, Creation: creation, Status: status, Done: done,
	}
}

func RestoreCommanderCell(stored StoredCommanderCell) (CommanderCell, error) {
	version, err := NewCellVersion(stored.Version)
	if err != nil {
		return CommanderCell{}, err
	}
	status, err := NewCellStatus(stored.Status)
	if err != nil {
		return CommanderCell{}, err
	}
	creationStatus, err := NewCreationStatus(string(stored.Creation.Status))
	if err != nil {
		return CommanderCell{}, err
	}
	stored.Creation.Status = creationStatus
	if stored.Creation.FailedStage != "" {
		if _, err := NewCreationStage(string(stored.Creation.FailedStage)); err != nil {
			return CommanderCell{}, err
		}
	}
	workspaceDriver, err := NewWorkspaceDriverType(string(stored.Workspace.Driver))
	if err != nil {
		return CommanderCell{}, err
	}
	windows := make([]WorkspaceWindow, 0, len(stored.Workspace.Windows))
	for _, window := range stored.Workspace.Windows {
		validated, err := NewWorkspaceWindow(window.Name, window.Command)
		if err != nil {
			return CommanderCell{}, err
		}
		windows = append(windows, validated)
	}
	workspace := NewWorkspace(workspaceDriver, windows)
	cell, err := NewCommanderCell(stored.ID, stored.Issue, stored.Project, stored.Template, workspace, stored.Targets, stored.Dependencies, stored.SourceDriver, stored.ContainerDriver, stored.NotificationDriver)
	if err != nil {
		return CommanderCell{}, err
	}
	cell.Version, cell.Note, cell.Creation, cell.Status, cell.Done = version, stored.Note, stored.Creation, status, stored.Done
	return cell, nil
}
