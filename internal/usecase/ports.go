package usecase

import (
	"context"

	"github.com/hgsg11/paracell/internal/domain"
)

type ConfigPort interface {
	Load(ctx context.Context, vars *domain.TemplateVars) (domain.Templates, error)
}

type InitConfigPort interface {
	ConfigExists(ctx context.Context) (bool, error)
	SaveConfig(ctx context.Context, cfg domain.Templates) error
}

type StateInitializer interface {
	Initialize(ctx context.Context) error
}

type CellStatePort interface {
	LoadCells(ctx context.Context) ([]domain.Cell, error)
	UpdateCells(ctx context.Context, update func([]domain.Cell) ([]domain.Cell, error)) error
}

type Notifier interface {
	NotifyReady(ctx context.Context, cell domain.Cell, message string) error
}

type NotificationProviderFactory interface {
	Notification(driver domain.NotificationDriverType) (Notifier, error)
}

type SourcePort interface {
	CreateSource(ctx context.Context, cell domain.Cell) (SourceCreation, error)
	ResumeSource(ctx context.Context, cell domain.Cell) error
	CleanSource(ctx context.Context, cell domain.Cell) error
}

type SourceCreation struct {
	BranchCreated bool
}

type SourceProviderFactory interface {
	Source(driver domain.SourceDriverType) (SourcePort, error)
}

type ContainerPort interface {
	CreateContainers(ctx context.Context, cell domain.Cell, templates []domain.ContainerTemplate) error
	CleanContainers(ctx context.Context, cell domain.Cell) error
}

type ContainerProviderFactory interface {
	Container(driver domain.ContainerDriverType) (ContainerPort, error)
}

type SessionPort interface {
	CreateSession(ctx context.Context, cell domain.Cell) error
	CleanSession(ctx context.Context, cell domain.Cell) error
	PrepareSession(ctx context.Context, cell domain.Cell) error
	UpdateStatusLabel(ctx context.Context, cell domain.Cell) error
	EnterSession(ctx context.Context, cell domain.Cell) error
	EnterRootSession(ctx context.Context, projectName string) error
	ExitSession(ctx context.Context) error
}

type SessionProviderFactory interface {
	Session(driver domain.SessionDriverType) (SessionPort, error)
}

type IDGenerator interface {
	NewID() string
}

type CellFactory interface {
	NewCell(id string, issue string, templateName string, sources []domain.SourceTemplate, containers []domain.ContainerTemplate, session domain.SessionTemplate, project string) (domain.Cell, error)
}
