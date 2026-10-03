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
	LoadCells(context.Context) (CellSet, error)
	UpdateCells(context.Context, func(CellSet) (CellSet, error)) error
}

// CellSet is a state snapshot of independent runtime Cells; the CommanderCell's
// ID references, rather than embeds, its targets and dependencies.
type CellSet struct {
	Commanders   []domain.CommanderCell
	Targets      []domain.TargetCell
	Dependencies []domain.DependencyCell
}

func NewCellSet(commanders []domain.CommanderCell, targets []domain.TargetCell, dependencies []domain.DependencyCell) CellSet {
	return CellSet{Commanders: append([]domain.CommanderCell(nil), commanders...), Targets: append([]domain.TargetCell(nil), targets...), Dependencies: append([]domain.DependencyCell(nil), dependencies...)}
}

type SourcePort interface {
	CreateSource(context.Context, domain.SourceResource) error
	CleanSource(context.Context, domain.SourceResource) error
}

type ContainerPort interface {
	CreateContainers(context.Context, domain.ContainerResources) (map[string][]string, error)
	CleanContainers(context.Context, domain.ContainerResources) error
}

type WorkspacePort interface {
	CreateWorkspace(context.Context, domain.WorkspaceResource) error
	CleanWorkspace(context.Context, domain.WorkspaceResource) error
	PrepareWorkspace(context.Context, domain.WorkspaceResource) error
	UpdateStatusLabel(context.Context, domain.WorkspaceResource) error
	EnterWorkspace(context.Context, domain.WorkspaceResource) error
	EnterRootWorkspace(context.Context, string) error
	ExitWorkspace(context.Context) error
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

type WorkspaceProviderFactory interface {
	Workspace(driver domain.WorkspaceDriverType) (WorkspacePort, error)
}

type IDGenerator interface {
	NewID() string
}
