package usecase

import (
	"context"
	"fmt"
	"os/exec"

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

// AgentHookPort installs the status hooks into an agent's configuration,
// adding only the hooks that are not installed yet.
type AgentHookPort interface {
	InstallStatusHooks(ctx context.Context, hooks []domain.AgentStatusHook) error
}

type CellPort interface {
	LoadCells(context.Context) (CellSet, error)
	UpdateCells(context.Context, func(CellSet) (CellSet, error)) error
}

// CellSet is a state snapshot of CellGroups and the runtime Cells associated
// with them by CellGroup ID.
type CellSet struct {
	Groups       []domain.CellGroup
	Commanders   []domain.CommanderCell
	Targets      []domain.TargetCell
	Dependencies []domain.DependencyCell
}

func NewCellSet(groups []domain.CellGroup, commanders []domain.CommanderCell, targets []domain.TargetCell, dependencies []domain.DependencyCell) CellSet {
	return CellSet{
		Groups:       append([]domain.CellGroup(nil), groups...),
		Commanders:   append([]domain.CommanderCell(nil), commanders...),
		Targets:      append([]domain.TargetCell(nil), targets...),
		Dependencies: append([]domain.DependencyCell(nil), dependencies...),
	}
}

// group returns the CellGroup with the given ID.
func (s CellSet) group(id string) (domain.CellGroup, bool) {
	for _, group := range s.Groups {
		if group.ID == id {
			return group, true
		}
	}
	return domain.CellGroup{}, false
}

// resolve finds a CommanderCell and its CellGroup by a user-facing identifier.
func (s CellSet) resolve(identifier string) (domain.CommanderCell, domain.CellGroup, error) {
	commander, ok := domain.ResolveCommanderCellService(s.Groups, s.Commanders, identifier)
	if !ok {
		return domain.CommanderCell{}, domain.CellGroup{}, fmt.Errorf("cell %q not found", identifier)
	}
	group, ok := s.group(commander.CellGroupID)
	if !ok {
		return domain.CommanderCell{}, domain.CellGroup{}, fmt.Errorf("cell %q not found", identifier)
	}
	return commander, group, nil
}

// replaceCommander replaces the CommanderCell that has the same ID.
func (s CellSet) replaceCommander(commander domain.CommanderCell) error {
	for i := range s.Commanders {
		if s.Commanders[i].ID == commander.ID {
			s.Commanders[i] = commander
			return nil
		}
	}
	return fmt.Errorf("CommanderCell %q not found", commander.ID)
}

// replaceGroup replaces the CellGroup that has the same ID.
func (s CellSet) replaceGroup(group domain.CellGroup) error {
	for i := range s.Groups {
		if s.Groups[i].ID == group.ID {
			s.Groups[i] = group
			return nil
		}
	}
	return fmt.Errorf("CellGroup %q not found", group.ID)
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
	// EnterCommand prepares the cell session and returns the command that moves this terminal to it.
	EnterCommand(context.Context, domain.WorkspaceResource) (*exec.Cmd, error)
	UpdateStatusLabel(context.Context, domain.WorkspaceResource) error
	EnterWorkspace(context.Context, domain.WorkspaceResource) error
	EnterRootWorkspace(context.Context, string) error
	ExitWorkspace(context.Context) error
}

type Notifier interface {
	NotifyReady(ctx context.Context, resource domain.WorkspaceResource, message string) error
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
