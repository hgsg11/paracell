package usecase

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/hgsg11/paracell/internal/domain"
)

type CleanCellInput struct {
	Cell string
}

type CleanCellUseCase struct {
	Cells            CellPort
	SourceFactory    SourceProviderFactory
	ContainerFactory ContainerProviderFactory
	WorkspaceFactory WorkspaceProviderFactory
}

func (u CleanCellUseCase) Execute(ctx context.Context, input CleanCellInput) error {
	cells, err := u.Cells.LoadCells(ctx)
	if err != nil {
		return err
	}
	commander, ok := domain.ResolveCommanderCell(cells.Commanders, input.Cell)
	if !ok {
		return fmt.Errorf("cell %q not found", input.Cell)
	}
	if err := commander.EnsureCanBeCleaned(); err != nil {
		return err
	}
	drivers := commander.ResourceDrivers()
	workspace, err := u.WorkspaceFactory.Workspace(drivers.Workspace)
	if err != nil {
		return err
	}
	containers, err := u.ContainerFactory.Container(drivers.Container)
	if err != nil {
		return err
	}
	source, err := u.SourceFactory.Source(drivers.Source)
	if err != nil {
		return err
	}
	targets := make([]domain.TargetCell, 0, len(commander.Targets))
	for _, targetID := range commander.Targets {
		for _, target := range cells.Targets {
			if target.ID == targetID {
				targets = append(targets, target)
				break
			}
		}
	}
	dependencies := make([]domain.DependencyCell, 0, len(commander.Dependencies))
	for _, dependencyID := range commander.Dependencies {
		for _, dependency := range cells.Dependencies {
			if dependency.ID == dependencyID {
				dependencies = append(dependencies, dependency)
				break
			}
		}
	}
	if err := ignoreNotFound(workspace.CleanWorkspace(ctx, commander.WorkspaceResource())); err != nil {
		return err
	}
	containerResources := commanderContainerResources(commander, targets, dependencies, nil)
	if err := ignoreNotFound(containers.CleanContainers(ctx, containerResources)); err != nil {
		return err
	}
	for _, resource := range commander.SourceResources(targets) {
		if err := ignoreNotFound(source.CleanSource(ctx, resource)); err != nil {
			return err
		}
	}
	return u.Cells.UpdateCells(ctx, func(latest CellSet) (CellSet, error) {
		commanders := make([]domain.CommanderCell, 0, len(latest.Commanders))
		for _, current := range latest.Commanders {
			if current.ID != commander.ID {
				commanders = append(commanders, current)
			}
		}
		remainingTargets := make([]domain.TargetCell, 0, len(latest.Targets))
		for _, target := range latest.Targets {
			if target.CommanderID != commander.ID {
				remainingTargets = append(remainingTargets, target)
			}
		}
		remainingDependencies := make([]domain.DependencyCell, 0, len(latest.Dependencies))
		for _, dependency := range latest.Dependencies {
			if dependency.CommanderID != commander.ID {
				remainingDependencies = append(remainingDependencies, dependency)
			}
		}
		return NewCellSet(commanders, remainingTargets, remainingDependencies), nil
	})
}

func commanderContainerResources(commander domain.CommanderCell, targets []domain.TargetCell, dependencies []domain.DependencyCell, templates map[string]domain.ContainerTemplate) domain.ContainerResources {
	items := make([]domain.ContainerResource, 0, len(targets)+len(dependencies))
	for _, target := range targets {
		if target.Container == nil {
			continue
		}
		template := templates[target.Container.SourceContainer]
		name := commander.ResourcePrefix() + "-" + domain.SafeResourceName(target.Name, "target") + "-" + domain.SafeResourceName(target.Container.SourceContainer, "container")
		item := domain.NewContainerResource(name, target.Container.Network, target.Container.SourceContainer, target.Container.Mode, template.Environments, template.Mounts)
		if target.Source != nil {
			item.SourcePath = commander.SourceWorktreePath(target.Name)
			if target.Source.Path != "." {
				item.SourcePath = filepath.Join(item.SourcePath, target.Source.Path)
			}
		}
		items = append(items, item)
	}
	for _, dependency := range dependencies {
		template := templates[dependency.Container.SourceContainer]
		items = append(items, domain.NewContainerResource(dependency.Container.SourceContainer, dependency.Container.Network, dependency.Container.SourceContainer, dependency.Container.Mode, template.Environments, template.Mounts))
	}
	return domain.NewContainerResources(commander.Name().Value, commander.Project, commander.ResourcePrefix(), items)
}

func ignoreNotFound(err error) error {
	if err == nil {
		return nil
	}
	type multiUnwrapper interface {
		Unwrap() []error
	}
	if joined, ok := err.(multiUnwrapper); ok {
		var remaining error
		for _, nested := range joined.Unwrap() {
			remaining = errors.Join(remaining, ignoreNotFound(nested))
		}
		return remaining
	}
	type singleUnwrapper interface {
		Unwrap() error
	}
	if wrapped, ok := err.(singleUnwrapper); ok && errors.Is(wrapped.Unwrap(), domain.ErrNotFound) {
		return ignoreNotFound(wrapped.Unwrap())
	}
	if errors.Is(err, domain.ErrNotFound) {
		return nil
	}
	return err
}
