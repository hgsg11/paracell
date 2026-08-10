package usecase

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/hgsg11/paracell/internal/domain"
)

const (
	defaultRetryHeartbeatInterval = 10 * time.Second
	defaultRetryLeaseTimeout      = 2 * time.Minute
)

type RetryCellInput struct {
	Cell string
}

type RetryCellUseCase struct {
	Config            ConfigPort
	State             CellStatePort
	CellFactory       CellFactory
	SourceFactory     SourceProviderFactory
	Files             FilePort
	ContainerFactory  ContainerProviderFactory
	SessionFactory    SessionProviderFactory
	IDs               IDGenerator
	Now               func() time.Time
	HeartbeatInterval time.Duration
	LeaseTimeout      time.Duration
}

func (u RetryCellUseCase) Execute(ctx context.Context, input RetryCellInput) (domain.Cell, error) {
	if u.IDs == nil {
		return domain.Cell{}, errors.New("retry attempt ID generator is required")
	}
	now := u.now()
	attemptID := u.IDs.NewID()
	if attemptID == "" {
		return domain.Cell{}, errors.New("retry attempt ID is empty")
	}
	cell, err := u.acquireRetry(ctx, input.Cell, attemptID, now)
	if err != nil {
		return domain.Cell{}, err
	}

	runCtx, heartbeat := u.startHeartbeat(ctx, cell, attemptID)
	failValidation := func(validationErr error) (domain.Cell, error) {
		heartbeatErr := heartbeat.stop()
		stage := cell.Creation.FailedStage
		if stage == "" {
			stage = nextCreationStage(cell)
		}
		cell.FailCreation(stage, validationErr)
		if saveErr := replaceRetryCell(context.WithoutCancel(ctx), u.State, cell, attemptID); saveErr != nil {
			return domain.Cell{}, errors.Join(validationErr, heartbeatErr, fmt.Errorf("save failed cell: %w", saveErr))
		}
		return domain.Cell{}, errors.Join(validationErr, heartbeatErr)
	}

	cfg, err := u.Config.Load(runCtx, &domain.TemplateVars{
		Issue:   cell.Issue,
		Name:    cell.Name,
		Command: cell.Creation.Command,
	})
	if err != nil {
		return failValidation(err)
	}
	template, ok := cfg.Templates[cell.Template]
	if !ok {
		return failValidation(fmt.Errorf("template %q not found", cell.Template))
	}
	rendered, err := u.CellFactory.NewCell(cell.ID, cell.Issue, template, cfg.Project.Name)
	if err != nil {
		return failValidation(err)
	}
	stored := cell
	cell = refreshRetryCell(cell, rendered)
	source, err := u.SourceFactory.Source(cfg.Providers)
	if err != nil {
		return failValidation(err)
	}
	containers, err := u.ContainerFactory.Container(cfg.Providers)
	if err != nil {
		return failValidation(err)
	}
	session, err := u.SessionFactory.Session(cfg.Providers)
	if err != nil {
		return failValidation(err)
	}

	runner := cellCreationRunner{
		State:          u.State,
		Files:          u.Files,
		Source:         source,
		Containers:     containers,
		Session:        session,
		RetryBase:      &stored,
		AttemptID:      attemptID,
		BeforeTerminal: heartbeat.stop,
	}
	runErr := runner.run(runCtx, &cell, template, true)
	heartbeatErr := heartbeat.stop()
	if runErr != nil || heartbeatErr != nil {
		return domain.Cell{}, errors.Join(runErr, heartbeatErr)
	}
	return cell, nil
}

func (u RetryCellUseCase) acquireRetry(ctx context.Context, identifier string, attemptID string, now time.Time) (domain.Cell, error) {
	var acquired domain.Cell
	err := u.State.UpdateCells(ctx, func(cells []domain.Cell) ([]domain.Cell, error) {
		cell, ok := resolveCell(cells, identifier)
		if !ok {
			return nil, fmt.Errorf("cell %q not found", identifier)
		}
		switch cell.CreationStatus() {
		case domain.CreationFailed:
		case domain.CreationRetrying:
			if cell.RetryLeaseValid(now, u.leaseTimeout()) {
				return nil, fmt.Errorf("retry already in progress for cell %q", cell.Name)
			}
		default:
			return nil, fmt.Errorf("cell %q is %s and cannot be retried", cell.Name, cell.CreationStatus())
		}
		cell.BeginRetry(attemptID, now)
		for index := range cells {
			if cells[index].ID == cell.ID {
				cells[index] = cell
				acquired = cloneCell(cell)
				return cells, nil
			}
		}
		return nil, fmt.Errorf("cell %q not found", identifier)
	})
	return acquired, err
}

func (u RetryCellUseCase) heartbeat(ctx context.Context, cell domain.Cell, attemptID string) error {
	return u.State.UpdateCells(ctx, func(cells []domain.Cell) ([]domain.Cell, error) {
		for index := range cells {
			if cells[index].ID != cell.ID {
				continue
			}
			if cells[index].CreationStatus() != domain.CreationRetrying || cells[index].Creation.AttemptID != attemptID {
				return nil, retryOwnershipLostError(cell.Name)
			}
			cells[index].HeartbeatRetry(u.now())
			return cells, nil
		}
		return nil, fmt.Errorf("cell %q not found", cell.ID)
	})
}

type retryHeartbeat struct {
	cancel context.CancelFunc
	done   <-chan error
	once   sync.Once
	err    error
}

func (u RetryCellUseCase) startHeartbeat(ctx context.Context, cell domain.Cell, attemptID string) (context.Context, *retryHeartbeat) {
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() {
		ticker := time.NewTicker(u.heartbeatInterval())
		defer ticker.Stop()
		for {
			select {
			case <-runCtx.Done():
				done <- nil
				return
			case <-ticker.C:
				if err := u.heartbeat(context.WithoutCancel(runCtx), cell, attemptID); err != nil {
					cancel()
					done <- err
					return
				}
			}
		}
	}()
	return runCtx, &retryHeartbeat{cancel: cancel, done: done}
}

func (h *retryHeartbeat) stop() error {
	h.once.Do(func() {
		h.cancel()
		h.err = <-h.done
	})
	return h.err
}

func (u RetryCellUseCase) now() time.Time {
	if u.Now != nil {
		return u.Now().UTC()
	}
	return time.Now().UTC()
}

func (u RetryCellUseCase) heartbeatInterval() time.Duration {
	if u.HeartbeatInterval > 0 {
		return u.HeartbeatInterval
	}
	return defaultRetryHeartbeatInterval
}

func (u RetryCellUseCase) leaseTimeout() time.Duration {
	if u.LeaseTimeout > 0 {
		return u.LeaseTimeout
	}
	return defaultRetryLeaseTimeout
}

func retryOwnershipLostError(cell string) error {
	return fmt.Errorf("retry ownership lost for cell %q", cell)
}

func resolveCell(cells []domain.Cell, identifier string) (domain.Cell, bool) {
	for _, cell := range cells {
		if cell.ID == identifier || cell.Issue == identifier || cell.Name == identifier {
			return cell, true
		}
	}
	return domain.Cell{}, false
}

func nextCreationStage(cell domain.Cell) domain.CreationStage {
	for _, stage := range []domain.CreationStage{
		domain.CreationStageSource,
		domain.CreationStageFiles,
		domain.CreationStageContainers,
		domain.CreationStageSession,
	} {
		if !cell.CreationStageCompleted(stage) {
			return stage
		}
	}
	return domain.CreationStageSession
}

func refreshRetryCell(stored domain.Cell, rendered domain.Cell) domain.Cell {
	refreshed := rendered
	refreshed.ID = stored.ID
	refreshed.Issue = stored.Issue
	refreshed.Name = stored.Name
	refreshed.Note = stored.Note
	refreshed.Template = stored.Template
	refreshed.Branch = stored.Branch
	refreshed.Source.Path = stored.Source.Path
	refreshed.Creation = stored.Creation
	if stored.CreationStageCompleted(domain.CreationStageSource) {
		refreshed.Base = stored.Base
		refreshed.BranchMode = stored.BranchMode
		refreshed.Source = stored.Source
	}
	if stored.CreationStageCompleted(domain.CreationStageContainers) {
		refreshed.Containers = stored.Containers
	} else {
		refreshed.Containers.Network = stored.Containers.Network
		for role, current := range stored.Containers.Services {
			updated, ok := refreshed.Containers.Services[role]
			if !ok {
				continue
			}
			updated.ContainerName = current.ContainerName
			refreshed.Containers.Services[role] = updated
		}
	}
	if stored.CreationStageCompleted(domain.CreationStageSession) {
		refreshed.Session = stored.Session
	} else {
		refreshed.Session.Name = stored.Session.Name
	}
	return refreshed
}
