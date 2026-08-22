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
	Files            FilePort
	ContainerFactory ContainerProviderFactory
	SessionFactory   SessionProviderFactory
	IDs              IDGenerator
}

func (u ForkCellUseCase) Execute(ctx context.Context, input ForkCellInput) (domain.Cell, error) {
	var note string
	if input.Note != nil {
		var err error
		note, err = domain.NormalizeCellNote(*input.Note)
		if err != nil {
			return domain.Cell{}, err
		}
	}
	cfg, err := u.Config.Load(ctx, &domain.TemplateVars{
		Issue:   input.Issue,
		Name:    input.Issue,
		Command: input.Command,
	})
	if err != nil {
		return domain.Cell{}, err
	}
	template, ok := cfg.Templates[input.Template]
	if !ok {
		if _, abstract := cfg.AbstractTemplates[input.Template]; abstract {
			return domain.Cell{}, fmt.Errorf("template %q is abstract and cannot be selected", input.Template)
		}
		return domain.Cell{}, fmt.Errorf("template %q not found", input.Template)
	}
	existing, err := u.State.LoadCells(ctx)
	if err != nil {
		return domain.Cell{}, err
	}
	cell, err := u.CellFactory.NewCell(u.IDs.NewID(), input.Issue, template, cfg.Project.Name)
	if err != nil {
		return domain.Cell{}, err
	}
	cell.Note = note
	if err := ensureForkUnique(existing, input.Issue, cell.Name); err != nil {
		return domain.Cell{}, err
	}
	source, err := u.SourceFactory.Source(cfg.Providers)
	if err != nil {
		return domain.Cell{}, err
	}
	containers, err := u.ContainerFactory.Container(cfg.Providers)
	if err != nil {
		return domain.Cell{}, err
	}
	session, err := u.SessionFactory.Session(cfg.Providers)
	if err != nil {
		return domain.Cell{}, err
	}

	cell.BeginCreation(input.Command)
	if err := u.State.UpdateCells(ctx, func(latest []domain.Cell) ([]domain.Cell, error) {
		if err := ensureForkUnique(latest, input.Issue, cell.Name); err != nil {
			return nil, err
		}
		return append(latest, cell), nil
	}); err != nil {
		return domain.Cell{}, err
	}

	runner := cellCreationRunner{
		State:      u.State,
		Files:      u.Files,
		Source:     source,
		Containers: containers,
		Session:    session,
	}
	if err := runner.run(ctx, &cell, template, false); err != nil {
		return domain.Cell{}, err
	}
	return cell, nil
}

func ensureForkUnique(existing []domain.Cell, issue string, name string) error {
	for _, cell := range existing {
		if cell.Issue != issue && cell.Name != name {
			continue
		}
		if cell.CreationStatus() == domain.CreationFailed {
			return fmt.Errorf("cell %q is failed; use paracell retry %s", cell.Name, cell.Name)
		}
		return (domain.CellUniquenessChecker{}).EnsureUnique(existing, issue, name)
	}
	return nil
}

type cellCreationRunner struct {
	State          CellStatePort
	Files          FilePort
	Source         SourcePort
	Containers     ContainerPort
	Session        SessionPort
	RetryBase      *domain.Cell
	AttemptID      string
	BeforeTerminal func() error
}

func (r cellCreationRunner) run(ctx context.Context, cell *domain.Cell, template domain.Template, retry bool) error {
	stages := []domain.CreationStage{
		domain.CreationStageSource,
		domain.CreationStageFiles,
		domain.CreationStageContainers,
		domain.CreationStageSession,
	}
	for _, stage := range stages {
		if cell.CreationStageCompleted(stage) {
			continue
		}
		before := cloneCell(*cell)
		if err := r.runStage(ctx, *cell, template, stage, retry); err != nil {
			*cell = before
			rollbackErr := r.rollbackSharedDatabaseContainers(context.WithoutCancel(ctx), cell, stage)
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
		if err := r.save(saveCtx, *cell); err != nil {
			terminalErr := r.beforeTerminal()
			cleanupErr := r.cleanupUncheckpointedStage(context.WithoutCancel(ctx), before, stage)
			*cell = before
			rollbackErr := r.rollbackSharedDatabaseContainers(context.WithoutCancel(ctx), cell, stage)
			return r.fail(ctx, cell, stage, errors.Join(fmt.Errorf("save %s checkpoint: %w", stage, err), terminalErr, cleanupErr, rollbackErr))
		}
	}
	return nil
}

func (r cellCreationRunner) rollbackSharedDatabaseContainers(ctx context.Context, cell *domain.Cell, failedStage domain.CreationStage) error {
	if failedStage != domain.CreationStageSession || !cell.CreationStageCompleted(domain.CreationStageContainers) || !cellUsesSharedDatabase(*cell) {
		return nil
	}
	err := ignoreNotFound(r.Containers.CleanContainers(ctx, *cell))
	cell.ResetCreationStage(domain.CreationStageContainers)
	return err
}

func cellUsesSharedDatabase(cell domain.Cell) bool {
	for _, service := range cell.Containers.Services {
		if service.Database != nil && service.Database.Mode == domain.DatabaseModeShared {
			return true
		}
	}
	return false
}

func (r cellCreationRunner) beforeTerminal() error {
	if r.BeforeTerminal == nil {
		return nil
	}
	return r.BeforeTerminal()
}

func (r cellCreationRunner) runStage(ctx context.Context, cell domain.Cell, template domain.Template, stage domain.CreationStage, retry bool) error {
	switch stage {
	case domain.CreationStageSource:
		if retry {
			return r.Source.ResumeSource(ctx, cell)
		}
		_, err := r.Source.CreateSource(ctx, cell)
		return err
	case domain.CreationStageFiles:
		if r.Files == nil {
			return nil
		}
		if retry {
			return r.Files.ResumeFiles(ctx, cell, templateWithInitFiles(template))
		}
		return r.Files.CopyFiles(ctx, cell, templateWithInitFiles(template))
	case domain.CreationStageContainers:
		if retry {
			cleanupCell := cell
			if r.RetryBase != nil {
				cleanupCell = *r.RetryBase
			}
			if err := ignoreNotFound(r.Containers.CleanContainers(ctx, cleanupCell)); err != nil {
				return fmt.Errorf("prepare containers retry: %w", err)
			}
		}
		return r.Containers.CreateContainers(ctx, cell, template)
	case domain.CreationStageSession:
		if retry {
			cleanupCell := cell
			if r.RetryBase != nil {
				cleanupCell = *r.RetryBase
			}
			if err := ignoreNotFound(r.Session.CleanSession(ctx, cleanupCell)); err != nil {
				return fmt.Errorf("prepare session retry: %w", err)
			}
		}
		return r.Session.CreateSession(ctx, cell)
	default:
		return fmt.Errorf("unsupported creation stage %q", stage)
	}
}

func (r cellCreationRunner) cleanupUncheckpointedStage(ctx context.Context, cell domain.Cell, stage domain.CreationStage) error {
	switch stage {
	case domain.CreationStageContainers:
		return ignoreNotFound(r.Containers.CleanContainers(ctx, cell))
	case domain.CreationStageSession:
		return ignoreNotFound(r.Session.CleanSession(ctx, cell))
	default:
		return nil
	}
}

func (r cellCreationRunner) fail(ctx context.Context, cell *domain.Cell, stage domain.CreationStage, createErr error) error {
	cell.FailCreation(stage, createErr)
	saveErr := r.save(context.WithoutCancel(ctx), *cell)
	if saveErr != nil {
		return errors.Join(createErr, fmt.Errorf("save failed cell: %w", saveErr))
	}
	return createErr
}

func (r cellCreationRunner) save(ctx context.Context, cell domain.Cell) error {
	if r.AttemptID != "" {
		return replaceRetryCell(ctx, r.State, cell, r.AttemptID)
	}
	return replaceCell(ctx, r.State, cell)
}

func replaceRetryCell(ctx context.Context, state CellStatePort, target domain.Cell, attemptID string) error {
	return state.UpdateCells(ctx, func(cells []domain.Cell) ([]domain.Cell, error) {
		for index := range cells {
			if cells[index].ID != target.ID {
				continue
			}
			if cells[index].CreationStatus() != domain.CreationRetrying || cells[index].Creation.AttemptID != attemptID {
				return nil, retryOwnershipLostError(target.Name)
			}
			if target.CreationStatus() == domain.CreationRetrying {
				target.Creation.LeaseStartedAt = cells[index].Creation.LeaseStartedAt
				target.Creation.LeaseHeartbeatAt = cells[index].Creation.LeaseHeartbeatAt
			}
			cells[index] = target
			return cells, nil
		}
		return nil, fmt.Errorf("cell %q not found", target.ID)
	})
}

func replaceCell(ctx context.Context, state CellStatePort, target domain.Cell) error {
	return state.UpdateCells(ctx, func(cells []domain.Cell) ([]domain.Cell, error) {
		for index := range cells {
			if cells[index].ID == target.ID {
				cells[index] = target
				return cells, nil
			}
		}
		return nil, fmt.Errorf("cell %q not found", target.ID)
	})
}

func cloneCell(cell domain.Cell) domain.Cell {
	cell.Creation.CompletedStages = append([]domain.CreationStage(nil), cell.Creation.CompletedStages...)
	return cell
}

func templateWithInitFiles(template domain.Template) domain.Template {
	merged := template
	files := append([]string(nil), template.Files...)
	seen := make(map[string]struct{}, len(files))
	for _, file := range files {
		seen[file] = struct{}{}
	}
	for _, service := range template.Containers.Services {
		if service.Database == nil {
			continue
		}
		for _, file := range service.Database.InitFiles {
			if _, ok := seen[file]; ok {
				continue
			}
			files = append(files, file)
			seen[file] = struct{}{}
		}
	}
	merged.Files = files
	return merged
}
