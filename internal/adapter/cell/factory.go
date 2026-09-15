package cell

import "github.com/hgsg11/paracell/internal/domain"

type Factory struct{}

func NewFactory() Factory {
	return Factory{}
}

func (Factory) NewCell(id string, issue string, project string, templateName string, sources domain.Sources, containers domain.Containers, session domain.Session, notificationDriver domain.NotificationDriverType) (domain.Cell, error) {
	return domain.NewCell(id, issue, project, templateName, sources, containers, session, notificationDriver)
}
