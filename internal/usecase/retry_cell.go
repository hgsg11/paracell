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

	retrySpec := domain.InspectCellRetry(cell)
	runCtx, heartbeat := u.startHeartbeat(ctx, cell, attemptID)
	failValidation := func(validationErr error) (domain.Cell, error) {
		heartbeatErr := heartbeat.stop()
		stage := retrySpec.FailedStage
		if stage == "" {
			stage = nextCreationStage(cell)
		}
		cell = domain.FailCellCreation(cell, stage, validationErr)
		if _, saveErr := replaceRetryCell(context.WithoutCancel(ctx), u.State, cell, attemptID); saveErr != nil {
			return domain.Cell{}, errors.Join(validationErr, heartbeatErr, fmt.Errorf("save failed cell: %w", saveErr))
		}
		return domain.Cell{}, errors.Join(validationErr, heartbeatErr)
	}

	cfg, err := u.Config.Load(runCtx)
	if err != nil {
		return failValidation(err)
	}
	resolved, err := domain.ResolveTemplate(cfg, retrySpec.Template, domain.NewTemplateVars(retrySpec.Issue, retrySpec.Name, retrySpec.Project, retrySpec.Command))
	if err != nil {
		return failValidation(err)
	}
	sources, err := domain.BuildSourcesForRetry(cell, cfg.SourceDriverType, resolved.Sources, retrySpec.Issue)
	if err != nil {
		return failValidation(err)
	}
	containers, err := domain.BuildContainersForRetry(cell, cfg.ContainerDriverType, resolved.Containers)
	if err != nil {
		return failValidation(err)
	}
	sessionEntity, err := domain.BuildSessionForRetry(cell, cfg.SessionDriverType, resolved.Session)
	if err != nil {
		return failValidation(err)
	}
	drivers := domain.CellResourceDrivers(cell)
	rendered, err := u.CellFactory.NewCell(retrySpec.ID, retrySpec.Issue, retrySpec.Project, retrySpec.Template, sources, containers, sessionEntity, drivers.Notification)
	if err != nil {
		return failValidation(err)
	}
	stored := cell
	cell = domain.RefreshCellForRetry(cell, rendered)
	drivers = domain.CellResourceDrivers(cell)
	source, err := u.SourceFactory.Source(drivers.Source)
	if err != nil {
		return failValidation(err)
	}
	containerPort, err := u.ContainerFactory.Container(drivers.Container)
	if err != nil {
		return failValidation(err)
	}
	sessionPort, err := u.SessionFactory.Session(drivers.Session)
	if err != nil {
		return failValidation(err)
	}

	runner := cellCreationRunner{
		State:          u.State,
		Source:         source,
		Containers:     containerPort,
		Session:        sessionPort,
		RetryBase:      &stored,
		AttemptID:      attemptID,
		BeforeTerminal: heartbeat.stop,
	}
	runErr := runner.run(runCtx, &cell, resolved.Containers, true)
	heartbeatErr := heartbeat.stop()
	if runErr != nil || heartbeatErr != nil {
		return domain.Cell{}, errors.Join(runErr, heartbeatErr)
	}
	return cell, nil
}

func (u RetryCellUseCase) acquireRetry(ctx context.Context, identifier string, attemptID string, now time.Time) (domain.Cell, error) {
	var acquired domain.Cell
	err := u.State.UpdateCells(ctx, func(cells []domain.Cell) ([]domain.Cell, error) {
		cell, ok := domain.ResolveCell(cells, identifier)
		if !ok {
			return nil, fmt.Errorf("cell %q not found", identifier)
		}
		switch domain.CellCreationStatus(cell) {
		case domain.CreationFailed:
		case domain.CreationRetrying:
			if domain.CellRetryLeaseValid(cell, now, u.leaseTimeout()) {
				return nil, fmt.Errorf("retry already in progress for cell %q", domain.CellName(cell))
			}
		default:
			return nil, fmt.Errorf("cell %q is %s and cannot be retried", domain.CellName(cell), domain.CellCreationStatus(cell))
		}
		cell = domain.BeginCellRetry(cell, attemptID, now)
		cellSummary := domain.SummarizeCell(cell)
		for index := range cells {
			if domain.SummarizeCell(cells[index]).ID == cellSummary.ID {
				cells[index] = cell
				acquired = domain.CloneCell(cell)
				return cells, nil
			}
		}
		return nil, fmt.Errorf("cell %q not found", identifier)
	})
	if err == nil {
		acquired = domain.CellAfterPersistence(acquired)
	}
	return acquired, err
}

func (u RetryCellUseCase) heartbeat(ctx context.Context, cell domain.Cell, attemptID string) error {
	summary := domain.SummarizeCell(cell)
	return u.State.UpdateCells(ctx, func(cells []domain.Cell) ([]domain.Cell, error) {
		for index := range cells {
			if domain.SummarizeCell(cells[index]).ID != summary.ID {
				continue
			}
			if !domain.CellRetryAttemptMatches(cells[index], attemptID) {
				return nil, retryOwnershipLostError(domain.CellName(cell))
			}
			cells[index] = domain.HeartbeatCellRetry(cells[index], u.now())
			return cells, nil
		}
		return nil, fmt.Errorf("cell %q not found", summary.ID)
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

func nextCreationStage(cell domain.Cell) domain.CreationStage {
	for _, stage := range []domain.CreationStage{
		domain.CreationStageSource,
		domain.CreationStageContainers,
		domain.CreationStageSession,
	} {
		if !domain.CellCreationStageCompleted(cell, stage) {
			return stage
		}
	}
	return domain.CreationStageSession
}
