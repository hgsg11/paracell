package domain

type CellDrivers struct {
	Source       SourceDriverType
	Container    ContainerDriverType
	Session      SessionDriverType
	Notification NotificationDriverType
}

func NewCellDrivers(source SourceDriverType, container ContainerDriverType, session SessionDriverType, notification NotificationDriverType) CellDrivers {
	return CellDrivers{Source: source, Container: container, Session: session, Notification: notification}
}
