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
	if err := domain.EnsureCommanderCellUnique(current.Commanders, input.Issue, name); err != nil {
		return domain.CommanderCell{}, err
	}
	commander, targets, dependencies, err := domain.CreateCellGroupService(cfg, resolved, input.Issue, commanderID, u.IDs)
	if err != nil {
		return domain.CommanderCell{}, err
	}
	if input.Note != nil {
		if err := commander.CellGroup.SetNote(*input.Note); err != nil {
			return domain.CommanderCell{}, err
		}
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
	commander.BeginCreation()
	cellSet := NewCellSet(append(current.Commanders, commander), append(current.Targets, targets...), append(current.Dependencies, dependencies...))
	if err := u.Cells.UpdateCells(ctx, func(latest CellSet) (CellSet, error) {
		if err := domain.EnsureCommanderCellUnique(latest.Commanders, input.Issue, name); err != nil {
			return CellSet{}, err
		}
		return NewCellSet(append(latest.Commanders, commander), append(latest.Targets, targets...), append(latest.Dependencies, dependencies...)), nil
	}); err != nil {
		return domain.CommanderCell{}, err
	}
	runner := cellCreationRunner{
		Cells: u.Cells, Source: source, Containers: containers, Workspace: workspace,
		Commander: &commander, Targets: targets, Dependencies: dependencies,
		ContainerTemplates: containerTemplates(*resolved.Commander),
	}
	if err := runner.run(ctx, &cellSet); err != nil {
		return domain.CommanderCell{}, err
	}
	return commander, nil
}

func containerTemplates(spec domain.CommanderCellSpec) map[string]domain.ContainerTemplate {
	templates := make(map[string]domain.ContainerTemplate, len(spec.Targets)+len(spec.Dependencies))
	for _, target := range spec.Targets {
		if target.Container != nil {
			templates[target.Container.Name] = *target.Container
		}
	}
	for _, dependency := range spec.Dependencies {
		templates[dependency.Container.Name] = dependency.Container
	}
	return templates
}

type cellCreationRunner struct {
	Cells              CellPort
	Source             SourcePort
	Containers         ContainerPort
	Workspace          WorkspacePort
	Commander          *domain.CommanderCell
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
			r.Commander.FinishCreation()
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
		for _, resource := range domain.BuildCellGroupSourcesService(*r.Commander, r.Targets) {
			if err := r.Source.CreateSource(ctx, resource); err != nil {
				return err
			}
		}
		return nil
	case domain.CreationStageContainers:
		resources := domain.BuildCellGroupContainersService(*r.Commander, r.Targets, r.Dependencies, r.ContainerTemplates)
		networks, err := r.Containers.CreateContainers(ctx, resources)
		if err != nil {
			return err
		}
		for i := range r.Targets {
			if r.Targets[i].Container != nil {
				r.Targets[i].Container.Network = append([]string(nil), networks[r.Targets[i].Container.SourceContainer]...)
			}
		}
		for i := range r.Dependencies {
			r.Dependencies[i].Container.Network = append([]string(nil), networks[r.Dependencies[i].Container.SourceContainer]...)
		}
		return nil
	case domain.CreationStageWorkspace:
		return r.Workspace.CreateWorkspace(ctx, r.Commander.WorkspaceResource())
	default:
		return fmt.Errorf("unsupported creation stage %q", stage)
	}
}

func (r cellCreationRunner) rollbackContainers(ctx context.Context, failedStage domain.CreationStage) error {
	if failedStage != domain.CreationStageWorkspace || r.Commander.ResourceDrivers().Container != domain.Docker {
		return nil
	}
	return ignoreNotFound(r.Containers.CleanContainers(ctx, domain.BuildCellGroupContainersService(*r.Commander, r.Targets, r.Dependencies, nil)))
}

func (r cellCreationRunner) cleanupUnpersistedStage(ctx context.Context, stage domain.CreationStage) error {
	if stage == domain.CreationStageWorkspace {
		return ignoreNotFound(r.Workspace.CleanWorkspace(ctx, r.Commander.WorkspaceResource()))
	}
	if stage == domain.CreationStageContainers {
		return ignoreNotFound(r.Containers.CleanContainers(ctx, domain.BuildCellGroupContainersService(*r.Commander, r.Targets, r.Dependencies, nil)))
	}
	return nil
}

func (r cellCreationRunner) fail(ctx context.Context, cells *CellSet, stage domain.CreationStage, createErr error) error {
	r.Commander.FailCreation(stage, createErr)
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
				for childIndex := range latest.Targets {
					if latest.Targets[childIndex].CellGroupID == r.Commander.CellGroup.ID {
						latest.Targets[childIndex] = targetByID(r.Targets, latest.Targets[childIndex].ID)
					}
				}
				for childIndex := range latest.Dependencies {
					if latest.Dependencies[childIndex].CellGroupID == r.Commander.CellGroup.ID {
						latest.Dependencies[childIndex] = dependencyByID(r.Dependencies, latest.Dependencies[childIndex].ID)
					}
				}
				*cells = NewCellSet(latest.Commanders, latest.Targets, latest.Dependencies)
				return latest, nil
			}
		}
		return CellSet{}, fmt.Errorf("CommanderCell %q not found", r.Commander.Name().Value)
	}); err != nil {
		return err
	}
	return r.Commander.AdvanceVersion()
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
