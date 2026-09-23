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
	SessionFactory   SessionProviderFactory
	IDs              IDGenerator
}

func (u ForkCellUseCase) Execute(ctx context.Context, input ForkCellInput) (domain.Cell, error) {
	cfg, err := u.Config.Load(ctx)
	if err != nil {
		return domain.Cell{}, err
	}
	id := u.IDs.NewID()
	name := domain.NewCellName(input.Issue)
	resolved, err := cfg.Resolve(input.Template, domain.NewTemplateVars(input.Issue, name.Value, cfg.ProjectName, input.Command))
	if err != nil {
		return domain.Cell{}, err
	}
	existing, err := u.Cells.LoadCells(ctx)
	if err != nil {
		return domain.Cell{}, err
	}
	sources, err := domain.BuildSources(cfg.SourceDriverType, resolved.Sources, input.Issue)
	if err != nil {
		return domain.Cell{}, err
	}
	containers, err := domain.BuildContainers(cfg.ContainerDriverType, resolved.Containers)
	if err != nil {
		return domain.Cell{}, err
	}
	session, err := domain.BuildSession(cfg.SessionDriverType, resolved.Session)
	if err != nil {
		return domain.Cell{}, err
	}
	cell, err := domain.NewCell(
		id, input.Issue, cfg.ProjectName, input.Template,
		sources,
		containers,
		session,
		cfg.NotificationDriverType,
	)
	if err != nil {
		return domain.Cell{}, err
	}
	source, err := u.SourceFactory.Source(cfg.SourceDriverType)
	if err != nil {
		return domain.Cell{}, err
	}
	containerPort, err := u.ContainerFactory.Container(cfg.ContainerDriverType)
	if err != nil {
		return domain.Cell{}, err
	}
	sessionPort, err := u.SessionFactory.Session(cfg.SessionDriverType)
	if err != nil {
		return domain.Cell{}, err
	}
	if input.Note != nil {
		if err = cell.SetNote(*input.Note); err != nil {
			return domain.Cell{}, err
		}
	}
	if err := domain.EnsureCellUnique(existing, input.Issue, cell.Name()); err != nil {
		return domain.Cell{}, err
	}
	cell.BeginCreation()
	if err := u.Cells.UpdateCells(ctx, func(latest []domain.Cell) ([]domain.Cell, error) {
		if err := domain.EnsureCellUnique(latest, input.Issue, cell.Name()); err != nil {
			return nil, err
		}
		return append(latest, cell), nil
	}); err != nil {
		return domain.Cell{}, err
	}

	runner := cellCreationRunner{
		Cells:      u.Cells,
		Source:     source,
		Containers: containerPort,
		Templates:  resolved,
		Session:    sessionPort,
	}
	if err := runner.run(ctx, &cell); err != nil {
		return domain.Cell{}, err
	}
	return cell, nil
}

type cellCreationRunner struct {
	Cells          CellPort
	Source         SourcePort
	Containers     ContainerPort
	Templates      domain.ResolvedTemplate
	Session        SessionPort
	BeforeTerminal func() error
}

func (r cellCreationRunner) run(ctx context.Context, cell *domain.Cell) error {
	stages := []domain.CreationStage{
		domain.CreationStageSource,
		domain.CreationStageContainers,
		domain.CreationStageSession,
	}
	for _, stage := range stages {
		if err := r.runStage(ctx, cell, stage); err != nil {
			rollbackErr := r.rollbackDependencyContainers(context.WithoutCancel(ctx), cell, stage)
			return r.fail(ctx, cell, stage, errors.Join(err, r.beforeTerminal(), rollbackErr))
		}
		if stage == domain.CreationStageSession {
			if err := r.beforeTerminal(); err != nil {
				return r.fail(ctx, cell, stage, err)
			}
			cell.FinishCreation()
		}
		saveCtx := ctx
		if stage == domain.CreationStageSession && r.BeforeTerminal != nil {
			saveCtx = context.WithoutCancel(ctx)
		}
		if err := r.save(saveCtx, cell); err != nil {
			terminalErr := r.beforeTerminal()
			cleanupErr := r.cleanupUnpersistedStage(context.WithoutCancel(ctx), *cell, stage)
			rollbackErr := r.rollbackDependencyContainers(context.WithoutCancel(ctx), cell, stage)
			return r.fail(ctx, cell, stage, errors.Join(fmt.Errorf("save %s stage: %w", stage, err), terminalErr, cleanupErr, rollbackErr))
		}
	}
	return nil
}

func (r cellCreationRunner) rollbackDependencyContainers(ctx context.Context, cell *domain.Cell, failedStage domain.CreationStage) error {
	if failedStage != domain.CreationStageSession || !cell.UsesDependency() {
		return nil
	}
	containers, dependencies := cell.ContainerCleanupTargets()
	return ignoreNotFound(r.Containers.CleanContainers(ctx, cell.ContainerNetworkName(), containers, dependencies))
}

func (r cellCreationRunner) beforeTerminal() error {
	if r.BeforeTerminal == nil {
		return nil
	}
	return r.BeforeTerminal()
}

func (r cellCreationRunner) runStage(ctx context.Context, cell *domain.Cell, stage domain.CreationStage) error {
	switch stage {
	case domain.CreationStageSource:
		return domain.CreateSourcesService(ctx, *cell, r.Templates.Sources, r.Source.CreateSource)
	case domain.CreationStageContainers:
		return domain.CreateContainersService(ctx, cell, r.Templates.Containers, r.Containers.CreateContainers)
	case domain.CreationStageSession:
		return domain.CreateSessionService(ctx, *cell, r.Templates.Session, r.Session.CreateSession)
	default:
		return fmt.Errorf("unsupported creation stage %q", stage)
	}
}

func (r cellCreationRunner) cleanupUnpersistedStage(ctx context.Context, cell domain.Cell, stage domain.CreationStage) error {
	switch stage {
	case domain.CreationStageContainers:
		containers, dependencies := cell.ContainerCleanupTargets()
		return ignoreNotFound(r.Containers.CleanContainers(ctx, cell.ContainerNetworkName(), containers, dependencies))
	case domain.CreationStageSession:
		return ignoreNotFound(r.Session.CleanSession(ctx, cell.SessionName()))
	default:
		return nil
	}
}

func (r cellCreationRunner) fail(ctx context.Context, cell *domain.Cell, stage domain.CreationStage, createErr error) error {
	cell.FailCreation(stage, createErr)
	saveErr := r.save(context.WithoutCancel(ctx), cell)
	if saveErr != nil {
		return errors.Join(createErr, fmt.Errorf("save failed cell: %w", saveErr))
	}
	return createErr
}

func (r cellCreationRunner) save(ctx context.Context, cell *domain.Cell) error {
	saved, err := replaceCell(ctx, r.Cells, *cell)
	if err == nil {
		if err := saved.AdvanceVersion(); err != nil {
			return err
		}
		*cell = saved
	}
	return err
}

func replaceCell(ctx context.Context, cellPort CellPort, target domain.Cell) (domain.Cell, error) {
	err := cellPort.UpdateCells(ctx, func(cells []domain.Cell) ([]domain.Cell, error) {
		for index := range cells {
			if cells[index].SameIdentity(target) {
				cells[index] = target
				return cells, nil
			}
		}
		return nil, fmt.Errorf("cell %q not found", target.Name().Value)
	})
	return target, err
}
