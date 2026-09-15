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
	State            CellStatePort
	CellFactory      CellFactory
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
	name := domain.SafeResourceName(input.Issue, id)
	resolved, err := domain.ResolveTemplate(cfg, input.Template, domain.NewTemplateVars(input.Issue, name, cfg.ProjectName, input.Command))
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
	sessionEntity, err := domain.BuildSession(cfg.SessionDriverType, resolved.Session)
	if err != nil {
		return domain.Cell{}, err
	}
	existing, err := u.State.LoadCells(ctx)
	if err != nil {
		return domain.Cell{}, err
	}
	cell, err := u.CellFactory.NewCell(id, input.Issue, cfg.ProjectName, input.Template, sources, containers, sessionEntity, cfg.NotificationDriverType)
	if err != nil {
		return domain.Cell{}, err
	}
	if input.Note != nil {
		if err = cell.SetNote(*input.Note); err != nil {
			return domain.Cell{}, err
		}
	}
	if err := ensureForkUnique(existing, input.Issue, cell.Name()); err != nil {
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

	cell.BeginCreation(input.Command)
	if err := u.State.UpdateCells(ctx, func(latest []domain.Cell) ([]domain.Cell, error) {
		if err := ensureForkUnique(latest, input.Issue, cell.Name()); err != nil {
			return nil, err
		}
		return append(latest, cell), nil
	}); err != nil {
		return domain.Cell{}, err
	}

	runner := cellCreationRunner{
		State:      u.State,
		Source:     source,
		Containers: containerPort,
		Session:    sessionPort,
	}
	if err := runner.run(ctx, &cell, resolved.Containers, false); err != nil {
		return domain.Cell{}, err
	}
	return cell, nil
}

func ensureForkUnique(existing []domain.Cell, issue string, name string) error {
	for _, cell := range existing {
		summary := cell.Summary()
		if summary.Issue != issue && summary.Name != name {
			continue
		}
		if cell.CreationStatus() == domain.CreationFailed {
			return fmt.Errorf("cell %q is failed; use paracell retry %s", cell.Name(), cell.Name())
		}
		return domain.EnsureCellUnique(existing, issue, name)
	}
	return nil
}

type cellCreationRunner struct {
	State          CellStatePort
	Source         domain.SourcePort
	Containers     domain.ContainerPort
	Session        domain.SessionPort
	RetryBase      *domain.Cell
	AttemptID      string
	BeforeTerminal func() error
}

func (r cellCreationRunner) run(ctx context.Context, cell *domain.Cell, templates []domain.ContainerTemplate, retry bool) error {
	stages := []domain.CreationStage{
		domain.CreationStageSource,
		domain.CreationStageContainers,
		domain.CreationStageSession,
	}
	for _, stage := range stages {
		if (*cell).CreationStageCompleted(stage) {
			continue
		}
		before := (*cell).Clone()
		if err := r.runStage(ctx, cell, templates, stage, retry); err != nil {
			*cell = before
			rollbackErr := r.rollbackDependencyContainers(context.WithoutCancel(ctx), cell, stage)
			return r.fail(ctx, cell, stage, errors.Join(err, r.beforeTerminal(), rollbackErr))
		}
		cell.CompleteCreationStage(stage)
		if stage == domain.CreationStageSession {
			if err := r.beforeTerminal(); err != nil {
				*cell = before
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
			cleanupErr := r.cleanupUncheckpointedStage(context.WithoutCancel(ctx), before, stage)
			*cell = before
			rollbackErr := r.rollbackDependencyContainers(context.WithoutCancel(ctx), cell, stage)
			return r.fail(ctx, cell, stage, errors.Join(fmt.Errorf("save %s checkpoint: %w", stage, err), terminalErr, cleanupErr, rollbackErr))
		}
	}
	return nil
}

func (r cellCreationRunner) rollbackDependencyContainers(ctx context.Context, cell *domain.Cell, failedStage domain.CreationStage) error {
	if failedStage != domain.CreationStageSession || !cell.CreationStageCompleted(domain.CreationStageContainers) || !cell.UsesDependency() {
		return nil
	}
	err := ignoreNotFound(domain.CleanContainers(ctx, *cell, r.Containers))
	cell.ResetCreationStage(domain.CreationStageContainers)
	return err
}

func (r cellCreationRunner) beforeTerminal() error {
	if r.BeforeTerminal == nil {
		return nil
	}
	return r.BeforeTerminal()
}

func (r cellCreationRunner) runStage(ctx context.Context, cell *domain.Cell, templates []domain.ContainerTemplate, stage domain.CreationStage, retry bool) error {
	switch stage {
	case domain.CreationStageSource:
		if retry {
			return domain.ResumeSources(ctx, *cell, r.Source)
		}
		_, err := domain.CreateSources(ctx, *cell, r.Source)
		return err
	case domain.CreationStageContainers:
		if retry {
			cleanupCell := *cell
			if r.RetryBase != nil {
				cleanupCell = *r.RetryBase
			}
			if err := ignoreNotFound(domain.CleanContainers(ctx, cleanupCell, r.Containers)); err != nil {
				return fmt.Errorf("prepare containers retry: %w", err)
			}
		}
		return domain.CreateContainers(ctx, cell, templates, r.Containers)
	case domain.CreationStageSession:
		if retry {
			cleanupCell := *cell
			if r.RetryBase != nil {
				cleanupCell = *r.RetryBase
			}
			if err := ignoreNotFound(domain.CleanSession(ctx, cleanupCell, r.Session)); err != nil {
				return fmt.Errorf("prepare session retry: %w", err)
			}
		}
		return domain.CreateSession(ctx, *cell, r.Session)
	default:
		return fmt.Errorf("unsupported creation stage %q", stage)
	}
}

func (r cellCreationRunner) cleanupUncheckpointedStage(ctx context.Context, cell domain.Cell, stage domain.CreationStage) error {
	switch stage {
	case domain.CreationStageContainers:
		return ignoreNotFound(domain.CleanContainers(ctx, cell, r.Containers))
	case domain.CreationStageSession:
		return ignoreNotFound(domain.CleanSession(ctx, cell, r.Session))
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
	var saved domain.Cell
	var err error
	if r.AttemptID != "" {
		saved, err = replaceRetryCell(ctx, r.State, *cell, r.AttemptID)
	} else {
		saved, err = replaceCell(ctx, r.State, *cell)
	}
	if err == nil {
		saved.AdvanceVersion()
		*cell = saved
	}
	return err
}

func replaceRetryCell(ctx context.Context, state CellStatePort, target domain.Cell, attemptID string) (domain.Cell, error) {
	targetSummary := target.Summary()
	err := state.UpdateCells(ctx, func(cells []domain.Cell) ([]domain.Cell, error) {
		for index := range cells {
			if cells[index].Summary().ID != targetSummary.ID {
				continue
			}
			if err := target.PrepareRetryPersistence(cells[index], attemptID); err != nil {
				return nil, err
			}
			cells[index] = target
			return cells, nil
		}
		return nil, fmt.Errorf("cell %q not found", targetSummary.ID)
	})
	return target, err
}

func replaceCell(ctx context.Context, state CellStatePort, target domain.Cell) (domain.Cell, error) {
	targetSummary := target.Summary()
	err := state.UpdateCells(ctx, func(cells []domain.Cell) ([]domain.Cell, error) {
		for index := range cells {
			if cells[index].Summary().ID == targetSummary.ID {
				cells[index] = target
				return cells, nil
			}
		}
		return nil, fmt.Errorf("cell %q not found", targetSummary.ID)
	})
	return target, err
}
