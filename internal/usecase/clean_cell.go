package usecase

import (
	"context"
	"errors"
	"fmt"

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
	targets, dependencies := domain.SelectCellGroupMembersService(commander.CellGroup.ID, cells.Targets, cells.Dependencies)
	if err := ignoreNotFound(workspace.CleanWorkspace(ctx, commander.WorkspaceResource())); err != nil {
		return err
	}
	containerResources := domain.BuildCellGroupContainersService(commander, targets, dependencies, nil)
	if err := ignoreNotFound(containers.CleanContainers(ctx, containerResources)); err != nil {
		return err
	}
	for _, resource := range domain.BuildCellGroupSourcesService(commander, targets) {
		if err := ignoreNotFound(source.CleanSource(ctx, resource)); err != nil {
			return err
		}
	}
	return u.Cells.UpdateCells(ctx, func(latest CellSet) (CellSet, error) {
		commanders := make([]domain.CommanderCell, 0, len(latest.Commanders))
		for _, current := range latest.Commanders {
			if current.CellGroup.ID != commander.CellGroup.ID {
				commanders = append(commanders, current)
			}
		}
		remainingTargets := make([]domain.TargetCell, 0, len(latest.Targets))
		for _, target := range latest.Targets {
			if target.CellGroupID != commander.CellGroup.ID {
				remainingTargets = append(remainingTargets, target)
			}
		}
		remainingDependencies := make([]domain.DependencyCell, 0, len(latest.Dependencies))
		for _, dependency := range latest.Dependencies {
			if dependency.CellGroupID != commander.CellGroup.ID {
				remainingDependencies = append(remainingDependencies, dependency)
			}
		}
		return NewCellSet(commanders, remainingTargets, remainingDependencies), nil
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
