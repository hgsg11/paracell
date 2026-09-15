package domain

import (
	"context"
	"path/filepath"
)

type SourceResource struct {
	RepositoryPath string
	WorktreePath   string
	Base           string
	Branch         string
}

func NewSourceResource(repositoryPath string, worktreePath string, base string, branch string) SourceResource {
	return SourceResource{RepositoryPath: repositoryPath, WorktreePath: worktreePath, Base: base, Branch: branch}
}

type SourcePort interface {
	CreateSource(context.Context, SourceResource) (bool, error)
	ResumeSource(context.Context, SourceResource) error
	CleanSource(context.Context, SourceResource) error
}

func CreateSources(ctx context.Context, cell Cell, port SourcePort) (bool, error) {
	created := false
	for _, source := range cell.Sources.Items {
		branchCreated, err := port.CreateSource(ctx, sourceResource(cell, source))
		created = created || branchCreated
		if err != nil {
			return created, err
		}
	}
	return created, nil
}

func ResumeSources(ctx context.Context, cell Cell, port SourcePort) error {
	for _, source := range cell.Sources.Items {
		if err := port.ResumeSource(ctx, sourceResource(cell, source)); err != nil {
			return err
		}
	}
	return nil
}

func CleanSources(ctx context.Context, cell Cell, port SourcePort) error {
	for _, source := range cell.Sources.Items {
		if err := port.CleanSource(ctx, sourceResource(cell, source)); err != nil {
			return err
		}
	}
	return nil
}

func sourceResource(cell Cell, source Source) SourceResource {
	return NewSourceResource(source.Path, SourceWorktreePath(cell, source), source.Base, source.Branch)
}

type ContainerResource struct {
	Role            string
	Name            string
	Network         []string
	SourceContainer string
	Mode            Mode
	Environments    []Environment
	Mounts          []Mount
}

func NewContainerResource(role string, name string, network []string, sourceContainer string, mode Mode, environments []Environment, mounts []Mount) ContainerResource {
	return ContainerResource{
		Role: role, Name: name, Network: append([]string(nil), network...), SourceContainer: sourceContainer,
		Mode: mode, Environments: append([]Environment(nil), environments...), Mounts: append([]Mount(nil), mounts...),
	}
}

type ContainerResources struct {
	CellName   string
	Project    string
	Network    string
	SourcePath string
	Items      []ContainerResource
}

func NewContainerResources(cellName string, project string, network string, sourcePath string, items []ContainerResource) ContainerResources {
	return ContainerResources{CellName: cellName, Project: project, Network: network, SourcePath: sourcePath, Items: append([]ContainerResource(nil), items...)}
}

type ContainerPort interface {
	CreateContainers(context.Context, ContainerResources) (map[string][]string, error)
	CleanContainers(context.Context, ContainerResources) error
}

func CreateContainers(ctx context.Context, cell Cell, templates []ContainerTemplate, port ContainerPort) (Cell, error) {
	networks, err := port.CreateContainers(ctx, containerResources(cell, templates))
	if err != nil {
		return cell, err
	}
	for index := range cell.Containers.Items {
		cell.Containers.Items[index].Network = append([]string(nil), networks[cell.Containers.Items[index].Role]...)
	}
	return cell, nil
}

func CleanContainers(ctx context.Context, cell Cell, port ContainerPort) error {
	return port.CleanContainers(ctx, containerResources(cell, nil))
}

func containerResources(cell Cell, templates []ContainerTemplate) ContainerResources {
	byRole := make(map[string]ContainerTemplate, len(templates))
	for _, template := range templates {
		byRole[template.Name] = template
	}
	items := make([]ContainerResource, 0, len(cell.Containers.Items))
	for _, container := range cell.Containers.Items {
		template := byRole[container.Role]
		items = append(items, NewContainerResource(
			container.Role, ContainerResourceName(cell, container), container.Network,
			container.SourceContainer, container.Mode, template.Environments, template.Mounts,
		))
	}
	sourcePath := ""
	if len(cell.Sources.Items) > 0 {
		sourcePath = SourceWorktreePath(cell, cell.Sources.Items[0])
		if sourcePath != "" {
			sourcePath = filepath.Clean(sourcePath)
		}
	}
	return NewContainerResources(CellName(cell), cell.Project, ContainerNetworkName(cell), sourcePath, items)
}

type SessionResource struct {
	Name             string
	CellName         string
	Project          string
	DisplayLabel     string
	WorkingDirectory string
	Windows          []SessionWindow
}

func NewSessionResource(name string, cellName string, project string, displayLabel string, workingDirectory string, windows []SessionWindow) SessionResource {
	return SessionResource{Name: name, CellName: cellName, Project: project, DisplayLabel: displayLabel, WorkingDirectory: workingDirectory, Windows: append([]SessionWindow(nil), windows...)}
}

type SessionPort interface {
	CreateSession(context.Context, SessionResource) error
	CleanSession(context.Context, SessionResource) error
	PrepareSession(context.Context, SessionResource) error
	UpdateStatusLabel(context.Context, SessionResource) error
	EnterSession(context.Context, SessionResource) error
	EnterRootSession(context.Context, string) error
	ExitSession(context.Context) error
}

func CreateSession(ctx context.Context, cell Cell, port SessionPort) error {
	return port.CreateSession(ctx, sessionResource(cell))
}

func CleanSession(ctx context.Context, cell Cell, port SessionPort) error {
	return port.CleanSession(ctx, sessionResource(cell))
}

func PrepareSession(ctx context.Context, cell Cell, port SessionPort) error {
	return port.PrepareSession(ctx, sessionResource(cell))
}

func UpdateSessionStatusLabel(ctx context.Context, cell Cell, port SessionPort) error {
	return port.UpdateStatusLabel(ctx, sessionResource(cell))
}

func EnterSession(ctx context.Context, cell Cell, port SessionPort) error {
	return port.EnterSession(ctx, sessionResource(cell))
}

func sessionResource(cell Cell) SessionResource {
	workingDirectory := ""
	if len(cell.Sources.Items) > 0 {
		workingDirectory = SourceWorktreePath(cell, cell.Sources.Items[0])
	}
	return NewSessionResource(SessionName(cell), CellName(cell), cell.Project, CellDisplayLabel(cell), workingDirectory, cell.Session.Windows)
}
