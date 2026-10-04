package domain

type CellDrivers struct {
	Source       SourceDriverType
	Container    ContainerDriverType
	Workspace    WorkspaceDriverType
	Notification NotificationDriverType
}

func NewCellDrivers(source SourceDriverType, container ContainerDriverType, workspace WorkspaceDriverType, notification NotificationDriverType) CellDrivers {
	return CellDrivers{Source: source, Container: container, Workspace: workspace, Notification: notification}
}
