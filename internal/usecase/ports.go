package usecase

import (
	"context"

	"github.com/hgsg11/paracell/internal/domain"
)

type ConfigPort interface {
	Load(ctx context.Context) (domain.Templates, error)
}

type InitConfigPort interface {
	ConfigExists(ctx context.Context) (bool, error)
	SaveConfig(ctx context.Context, cfg domain.Templates) error
}

type Notifier interface {
	NotifyReady(ctx context.Context, sessionName string, message string) error
}

type NotificationProviderFactory interface {
	Notification(driver domain.NotificationDriverType) (Notifier, error)
}

type SourceProviderFactory interface {
	Source(driver domain.SourceDriverType) (domain.SourcePort, error)
}

type ContainerProviderFactory interface {
	Container(driver domain.ContainerDriverType) (domain.ContainerPort, error)
}

type SessionProviderFactory interface {
	Session(driver domain.SessionDriverType) (domain.SessionPort, error)
}

type IDGenerator interface {
	NewID() string
}

type CellFactory interface {
	NewCell(id string, issue string, project string, templateName string, sources domain.Sources, containers domain.Containers, session domain.Session, notificationDriver domain.NotificationDriverType) (domain.Cell, error)
}
