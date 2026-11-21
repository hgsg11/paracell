package usecase

import (
	"context"
	"errors"

	"github.com/hgsg11/paracell/internal/domain"
)

type CleanCellInput struct {
	Cell string
}

type CleanCellUseCase struct {
	Cells            CellPort
	SourceFactory    SourceProviderFactory
	CellDirectory    CellDirectoryPort
	ContainerFactory ContainerProviderFactory
	WorkspaceFactory WorkspaceProviderFactory
}

func (u CleanCellUseCase) Execute(ctx context.Context, input CleanCellInput) error {
	cells, err := u.Cells.LoadCells(ctx)
	if err != nil {
		return err
	}
	commander, group, err := cells.resolve(input.Cell)
	if err != nil {
		return err
	}
	if err := commander.EnsureCanBeCleaned(); err != nil {
		return err
	}
	drivers := group.ResourceDrivers(commander.Workspace.Driver)
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
	targets, dependencies := domain.SelectCellGroupMembersService(group.ID, cells.Targets, cells.Dependencies)
	if err := ignoreNotFound(workspace.CleanWorkspace(ctx, group.WorkspaceResource(commander.Workspace))); err != nil {
		return err
	}
	containerResources := domain.BuildContainerResourcesService(group, targets, dependencies, nil)
	if err := ignoreNotFound(containers.CleanContainers(ctx, containerResources)); err != nil {
		return err
	}
	// Nested worktrees go first. The Cell directory, with the copied workspace
	// files, is removed only after every worktree is gone, so a worktree that
	// failed to be removed keeps its work.
	sources := domain.BuildSourceResourcesService(group, targets)
	for i := len(sources) - 1; i >= 0; i-- {
		if err := ignoreNotFound(source.CleanSource(ctx, sources[i])); err != nil {
			return err
		}
	}
	if len(sources) > 0 {
		if err := u.CellDirectory.RemoveCellDirectory(ctx, domain.BuildCellResourceLayoutService(group, targets).CellDirectory()); err != nil {
			return err
		}
	}
	return u.Cells.UpdateCells(ctx, func(latest CellSet) (CellSet, error) {
		groups := make([]domain.CellGroup, 0, len(latest.Groups))
		for _, current := range latest.Groups {
			if current.ID != group.ID {
				groups = append(groups, current)
			}
		}
		commanders := make([]domain.CommanderCell, 0, len(latest.Commanders))
		for _, current := range latest.Commanders {
			if current.CellGroupID != group.ID {
				commanders = append(commanders, current)
			}
		}
		remainingTargets := make([]domain.TargetCell, 0, len(latest.Targets))
		for _, target := range latest.Targets {
			if target.CellGroupID != group.ID {
				remainingTargets = append(remainingTargets, target)
			}
		}
		remainingDependencies := make([]domain.DependencyCell, 0, len(latest.Dependencies))
		for _, dependency := range latest.Dependencies {
			if dependency.CellGroupID != group.ID {
				remainingDependencies = append(remainingDependencies, dependency)
			}
		}
		return NewCellSet(groups, commanders, remainingTargets, remainingDependencies), nil
	})
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
