package usecase

import (
	"context"

	"github.com/hgsg11/paracell/internal/domain"
)

type ConfigPort interface {
	Load(ctx context.Context, vars *domain.TemplateVars) (domain.Config, error)
}

type InitConfigPort interface {
	ConfigExists(ctx context.Context) (bool, error)
	SaveConfig(ctx context.Context, cfg InitConfig) error
}

type CellStatePort interface {
	LoadCells(ctx context.Context) ([]domain.Cell, error)
	UpdateCells(ctx context.Context, update func([]domain.Cell) ([]domain.Cell, error)) error
}

type Notifier interface {
	NotifyReady(ctx context.Context, cell domain.Cell, message string) error
}

type NotificationProviderFactory interface {
	Notification(provider domain.ProviderConfig) (Notifier, error)
}

type SourcePort interface {
	CreateSource(ctx context.Context, cell domain.Cell) (SourceCreation, error)
	RollbackSource(ctx context.Context, cell domain.Cell, creation SourceCreation) error
	CleanSource(ctx context.Context, cell domain.Cell) error
}

type SourceCreation struct {
	BranchCreated bool
}

type SourceProviderFactory interface {
	Source(provider domain.ProviderConfig) (SourcePort, error)
}

type FilePort interface {
	CopyFiles(ctx context.Context, cell domain.Cell, template domain.Template) error
}

type ContainerPort interface {
	CreateContainers(ctx context.Context, cell domain.Cell, template domain.Template) error
	CleanContainers(ctx context.Context, cell domain.Cell) error
}

type ContainerProviderFactory interface {
	Container(provider domain.ProviderConfig) (ContainerPort, error)
}

type SessionPort interface {
	CreateSession(ctx context.Context, cell domain.Cell) error
	CleanSession(ctx context.Context, cell domain.Cell) error
	PrepareSession(ctx context.Context, cell domain.Cell) error
	EnterSession(ctx context.Context, cell domain.Cell) error
	EnterRootSession(ctx context.Context, project domain.ProjectConfig) error
	ExitSession(ctx context.Context) error
}

type SessionProviderFactory interface {
	Session(provider domain.ProviderConfig) (SessionPort, error)
}

type IDGenerator interface {
	NewID() string
}

type CellFactory interface {
	NewCell(id string, issue string, template domain.Template, project string) (domain.Cell, error)
}
