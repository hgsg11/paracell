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

type CellInitializer interface {
	Initialize(context.Context) error
}

type CellPort interface {
	LoadCells(context.Context) ([]domain.Cell, error)
	UpdateCells(context.Context, func([]domain.Cell) ([]domain.Cell, error)) error
	DeleteCell(context.Context, domain.Cell) error
}

type SourcePort interface {
	CreateSource(context.Context, domain.SourceResource) error
	CleanSource(context.Context, domain.SourceResource) error
}

type ContainerPort interface {
	CreateContainers(context.Context, domain.ContainerResources) (map[string][]string, error)
	CleanContainers(context.Context, domain.ContainerResources) error
}

type SessionPort interface {
	CreateSession(context.Context, domain.SessionResource) error
	CleanSession(context.Context, domain.SessionResource) error
	PrepareSession(context.Context, domain.SessionResource) error
	UpdateStatusLabel(context.Context, domain.SessionResource) error
	EnterSession(context.Context, domain.SessionResource) error
	EnterRootSession(context.Context, string) error
	ExitSession(context.Context) error
}

type Notifier interface {
	NotifyReady(ctx context.Context, sessionName string, message string) error
}

type NotificationProviderFactory interface {
	Notification(driver domain.NotificationDriverType) (Notifier, error)
}

type SourceProviderFactory interface {
	Source(driver domain.SourceDriverType) (SourcePort, error)
}

type ContainerProviderFactory interface {
	Container(driver domain.ContainerDriverType) (ContainerPort, error)
}

type SessionProviderFactory interface {
	Session(driver domain.SessionDriverType) (SessionPort, error)
}

type IDGenerator interface {
	NewID() string
}

type CellFactory interface {
	NewCell(id string, issue string, project string, templateName string, sources domain.Sources, containers domain.Containers, session domain.Session, notificationDriver domain.NotificationDriverType) (domain.Cell, error)
}
