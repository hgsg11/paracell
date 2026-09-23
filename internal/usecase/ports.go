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
	CreateSource(ctx context.Context, template domain.SourceTemplate, worktree string, branch string) error
	CleanSource(ctx context.Context, repository string, worktree string) error
}

type ContainerPort interface {
	CreateContainers(ctx context.Context, templates []domain.ContainerTemplate, cellName string, project string, network string, sourcePath string) (map[string][]string, error)
	CleanContainers(ctx context.Context, network string, containers []string, dependencies []string) error
}

type SessionPort interface {
	CreateSession(ctx context.Context, template domain.SessionTemplate, name string, cellName string, project string, label string, workingDirectory string) error
	CleanSession(ctx context.Context, name string) error
	PrepareSession(ctx context.Context, name string, cellName string, project string, label string, windowNames []string) error
	UpdateStatusLabel(ctx context.Context, name string, label string) error
	EnterSession(ctx context.Context, name string, cellName string, project string, label string, windowNames []string) error
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
