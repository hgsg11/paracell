package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/hgsg11/paracell/internal/domain"
)

type ForkCellInput struct {
	Issue    string
	Template string
	Command  string
	Note     *string
}

type ForkCellUseCase struct {
	Config           ConfigPort
	Cells            CellPort
	SourceFactory    SourceProviderFactory
	ContainerFactory ContainerProviderFactory
	WorkspaceFactory WorkspaceProviderFactory
	IDs              IDGenerator
}

func (u ForkCellUseCase) Execute(ctx context.Context, input ForkCellInput) (domain.CommanderCell, error) {
	cfg, err := u.Config.Load(ctx)
	if err != nil {
		return domain.CommanderCell{}, err
	}
	commanderID := u.IDs.NewID()
	name := domain.NewCellName(input.Issue)
	resolved, err := cfg.Resolve(input.Template, domain.NewTemplateVars(input.Issue, name.Value, cfg.ProjectName, input.Command))
	if err != nil {
		return domain.CommanderCell{}, err
	}
	if resolved.Commander == nil {
		return domain.CommanderCell{}, fmt.Errorf("template %q does not define a CommanderCell", input.Template)
	}
	current, err := u.Cells.LoadCells(ctx)
	if err != nil {
		return domain.CommanderCell{}, err
	}
	if err := domain.EnsureUniqueCellGroupIssueService(current.Groups, input.Issue); err != nil {
		return domain.CommanderCell{}, err
	}
	group, err := domain.NewCellGroup(u.IDs.NewID(), input.Issue, cfg.ProjectName, resolved.Name, cfg.SourceDriverType, cfg.ContainerDriverType, cfg.NotificationDriverType)
	if err != nil {
		return domain.CommanderCell{}, err
	}
	if input.Note != nil {
		if err := group.SetNote(*input.Note); err != nil {
			return domain.CommanderCell{}, err
		}
	}
	commander, targets, dependencies, err := domain.InstantiateCellsService(group.ID, group.Issue, *resolved.Commander, resolved.Targets, resolved.Dependencies, cfg.WorkspaceDriverType, commanderID, u.IDs)
	if err != nil {
		return domain.CommanderCell{}, err
	}
	source, err := u.SourceFactory.Source(cfg.SourceDriverType)
	if err != nil {
		return domain.CommanderCell{}, err
	}
	containers, err := u.ContainerFactory.Container(cfg.ContainerDriverType)
	if err != nil {
		return domain.CommanderCell{}, err
	}
	workspace, err := u.WorkspaceFactory.Workspace(cfg.WorkspaceDriverType)
	if err != nil {
		return domain.CommanderCell{}, err
	}
	group.BeginCreation()
	cellSet := NewCellSet(append(current.Commanders, commander), append(current.Groups, group), append(current.Targets, targets...), append(current.Dependencies, dependencies...))
	if err := u.Cells.UpdateCells(ctx, func(latest CellSet) (CellSet, error) {
		if err := domain.EnsureUniqueCellGroupIssueService(latest.Groups, input.Issue); err != nil {
			return CellSet{}, err
		}
		return NewCellSet(append(latest.Commanders, commander), append(latest.Groups, group), append(latest.Targets, targets...), append(latest.Dependencies, dependencies...)), nil
	}); err != nil {
		return domain.CommanderCell{}, err
	}
	runner := cellCreationRunner{
		Cells: u.Cells, Source: source, Containers: containers, Workspace: workspace,
		Commander: &commander, Group: &group, Targets: targets, Dependencies: dependencies,
		ContainerTemplates: containerTemplates(resolved.Targets),
	}
	if err := runner.run(ctx, &cellSet); err != nil {
		return domain.CommanderCell{}, err
	}
	return commander, nil
}

func containerTemplates(targets []domain.TargetCellSpec) map[string]domain.ContainerTemplate {
	templates := make(map[string]domain.ContainerTemplate)
	for _, target := range targets {
		for _, container := range target.Containers {
			templates[container.Name] = container
		}
	}
	return templates
}

type cellCreationRunner struct {
	Cells              CellPort
	Source             SourcePort
	Containers         ContainerPort
	Workspace          WorkspacePort
	Commander          *domain.CommanderCell
	Group              *domain.CellGroup
	Targets            []domain.TargetCell
	Dependencies       []domain.DependencyCell
	ContainerTemplates map[string]domain.ContainerTemplate
	BeforeTerminal     func() error
}

func (r cellCreationRunner) run(ctx context.Context, cells *CellSet) error {
	stages := []domain.CreationStage{domain.CreationStageSource, domain.CreationStageContainers, domain.CreationStageWorkspace}
	for _, stage := range stages {
		if err := r.runStage(ctx, stage); err != nil {
			rollbackErr := r.rollbackContainers(context.WithoutCancel(ctx), stage)
			return r.fail(ctx, cells, stage, errors.Join(err, rollbackErr))
		}
		if stage == domain.CreationStageWorkspace {
			if r.BeforeTerminal != nil {
				if err := r.BeforeTerminal(); err != nil {
					return r.fail(ctx, cells, stage, err)
				}
			}
			r.Group.FinishCreation()
		}
		saveCtx := ctx
		if stage == domain.CreationStageWorkspace && r.BeforeTerminal != nil {
			saveCtx = context.WithoutCancel(ctx)
		}
		if err := r.save(saveCtx, cells); err != nil {
			cleanupErr := r.cleanupUnpersistedStage(context.WithoutCancel(ctx), stage)
			rollbackErr := r.rollbackContainers(context.WithoutCancel(ctx), stage)
			return r.fail(ctx, cells, stage, errors.Join(fmt.Errorf("save %s stage: %w", stage, err), cleanupErr, rollbackErr))
		}
	}
	return nil
}

func (r cellCreationRunner) runStage(ctx context.Context, stage domain.CreationStage) error {
	switch stage {
	case domain.CreationStageSource:
		for _, resource := range domain.BuildSourceResourcesService(*r.Group, r.Targets) {
			if err := r.Source.CreateSource(ctx, resource); err != nil {
				return err
			}
		}
		return nil
	case domain.CreationStageContainers:
		resources := domain.BuildContainerResourcesService(*r.Group, r.Targets, r.Dependencies, r.ContainerTemplates)
		networks, err := r.Containers.CreateContainers(ctx, resources)
		if err != nil {
			return err
		}
		for i := range r.Targets {
			for j := range r.Targets[i].Containers {
				container := r.Targets[i].Containers[j]
				container.Network = append([]string(nil), networks[container.SourceContainer]...)
			}
		}
		for i := range r.Dependencies {
			r.Dependencies[i].Container.Network = append([]string(nil), networks[r.Dependencies[i].Container.SourceContainer]...)
		}
		return nil
	case domain.CreationStageWorkspace:
		return r.Workspace.CreateWorkspace(ctx, r.Group.WorkspaceResource(r.Commander.Workspace))
	default:
		return fmt.Errorf("unsupported creation stage %q", stage)
	}
}

func (r cellCreationRunner) rollbackContainers(ctx context.Context, failedStage domain.CreationStage) error {
	if failedStage != domain.CreationStageWorkspace || r.Group.ContainerDriver != domain.Docker {
		return nil
	}
	return ignoreNotFound(r.Containers.CleanContainers(ctx, domain.BuildContainerResourcesService(*r.Group, r.Targets, r.Dependencies, nil)))
}

func (r cellCreationRunner) cleanupUnpersistedStage(ctx context.Context, stage domain.CreationStage) error {
	if stage == domain.CreationStageWorkspace {
		return ignoreNotFound(r.Workspace.CleanWorkspace(ctx, r.Group.WorkspaceResource(r.Commander.Workspace)))
	}
	if stage == domain.CreationStageContainers {
		return ignoreNotFound(r.Containers.CleanContainers(ctx, domain.BuildContainerResourcesService(*r.Group, r.Targets, r.Dependencies, nil)))
	}
	return nil
}

func (r cellCreationRunner) fail(ctx context.Context, cells *CellSet, stage domain.CreationStage, createErr error) error {
	r.Group.FailCreation(stage, createErr)
	if err := r.save(context.WithoutCancel(ctx), cells); err != nil {
		return errors.Join(createErr, fmt.Errorf("save failed CommanderCell: %w", err))
	}
	return createErr
}

func (r cellCreationRunner) save(ctx context.Context, cells *CellSet) error {
	if err := r.Cells.UpdateCells(ctx, func(latest CellSet) (CellSet, error) {
		for index := range latest.Commanders {
			if latest.Commanders[index].ID == r.Commander.ID {
				latest.Commanders[index] = r.Commander.Clone()
				for groupIndex := range latest.Groups {
					if latest.Groups[groupIndex].ID == r.Group.ID {
						latest.Groups[groupIndex] = *r.Group
						break
					}
				}
				for childIndex := range latest.Targets {
					if latest.Targets[childIndex].CellGroupID == r.Group.ID {
						latest.Targets[childIndex] = targetByID(r.Targets, latest.Targets[childIndex].ID)
					}
				}
				for childIndex := range latest.Dependencies {
					if latest.Dependencies[childIndex].CellGroupID == r.Group.ID {
						latest.Dependencies[childIndex] = dependencyByID(r.Dependencies, latest.Dependencies[childIndex].ID)
					}
				}
				return NewCellSet(latest.Commanders, latest.Groups, latest.Targets, latest.Dependencies), nil
			}
		}
		return CellSet{}, fmt.Errorf("CommanderCell %q not found", r.Group.Name().Value)
	}); err != nil {
		return err
	}
	// Unchanged stages do not advance the persisted version. Use the committed
	// snapshot instead of assuming every save increments it.
	stored, err := r.Cells.LoadCells(ctx)
	if err != nil {
		return err
	}
	commander, ok := domain.ResolveCommanderCell(stored.Commanders, r.Commander.ID)
	if !ok {
		return fmt.Errorf("CommanderCell %q not found", r.Commander.ID)
	}
	*r.Commander = commander
	*cells = stored
	return nil
}

func targetByID(targets []domain.TargetCell, id string) domain.TargetCell {
	for _, target := range targets {
		if target.ID == id {
			return target
		}
	}
	return domain.TargetCell{}
}

func dependencyByID(dependencies []domain.DependencyCell, id string) domain.DependencyCell {
	for _, dependency := range dependencies {
		if dependency.ID == id {
			return dependency
		}
	}
	return domain.DependencyCell{}
}
