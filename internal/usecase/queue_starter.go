package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/hgsg11/paracell/internal/domain"
)

// queueStarter forks the queued entries that no longer wait for anyone.
type queueStarter struct {
	Cells CellPort
	Fork  ForkCellUseCase
}

// StartReleased forks every waiting entry without blockers, in Sequence order,
// and returns the entries it started. When two processes start the same entry,
// the issue uniqueness check in fork fails the second. A failed fork leaves its
// CellGroup failed, so the entry counts as running until it is abandoned.
func (s queueStarter) StartReleased(ctx context.Context) ([]domain.QueueEntry, error) {
	set, err := s.Cells.LoadCells(ctx)
	if err != nil {
		return nil, err
	}
	var started []domain.QueueEntry
	var startErr error
	for _, entry := range domain.SelectStartableQueueEntriesService(set.Queue, set.Groups) {
		var note *string
		if entry.Request.Note != "" {
			value := entry.Request.Note
			note = &value
		}
		if _, err := s.Fork.Execute(ctx, ForkCellInput{
			Issue: entry.Issue, Template: entry.Request.Template, Prefix: entry.Request.Prefix,
			Command: entry.Request.Command, Note: note, QueueEntryID: entry.ID,
		}); err != nil {
			startErr = errors.Join(startErr, fmt.Errorf("start queued cell %q: %w", entry.Issue, err))
			continue
		}
		started = append(started, entry)
	}
	return started, startErr
}

// finishCommander moves the resolved Cell to merged or abandoned and removes
// the QueueEntry it started from in the same state update, then starts the
// entries that were waiting for it.
func finishCommander(ctx context.Context, cells CellPort, fork ForkCellUseCase, identifier string, change func(*domain.CommanderCell) error) (domain.CommanderCell, error) {
	var updated domain.CommanderCell
	if err := cells.UpdateCells(ctx, func(set CellSet) (CellSet, error) {
		commander, group, err := set.resolve(identifier)
		if err != nil {
			return CellSet{}, err
		}
		if err := change(&commander); err != nil {
			return CellSet{}, err
		}
		if err := set.replaceCommander(commander); err != nil {
			return CellSet{}, err
		}
		// An entry that is already gone has nothing left to release.
		if group.QueueEntryID != "" {
			if err := set.Queue.Remove(group.QueueEntryID); err != nil && !errors.Is(err, domain.ErrNotFound) {
				return CellSet{}, err
			}
		}
		updated = commander
		return set, nil
	}); err != nil {
		return domain.CommanderCell{}, err
	}
	if _, err := (queueStarter{Cells: cells, Fork: fork}).StartReleased(ctx); err != nil {
		return updated, fmt.Errorf("cell is %s, but queued cells failed to start: %w", updated.Progress, err)
	}
	return updated, nil
}
