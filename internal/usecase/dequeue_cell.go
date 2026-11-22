package usecase

import (
	"context"
	"fmt"

	"github.com/hgsg11/paracell/internal/domain"
)

type DequeueCellInput struct {
	Issue string
}

type DequeueCellUseCase struct {
	Cells CellPort
	// Fork starts the entries that waited only for the removed one.
	Fork ForkCellUseCase
}

// Execute removes a waiting entry and starts the entries it released. A running
// entry, one a CellGroup refers to, is released only by merged or abandon.
func (u DequeueCellUseCase) Execute(ctx context.Context, input DequeueCellInput) (domain.QueueEntry, error) {
	var removed domain.QueueEntry
	if err := u.Cells.UpdateCells(ctx, func(latest CellSet) (CellSet, error) {
		entry, ok := latest.Queue.EntryForIssue(input.Issue)
		if !ok {
			return CellSet{}, fmt.Errorf("queue entry for issue %q not found", input.Issue)
		}
		if group, running := domain.FindQueueEntryCellGroupService(latest.Groups, entry.ID); running {
			return CellSet{}, fmt.Errorf("queue entry %q is already running as cell %s; use paracell merged or paracell abandon", input.Issue, group.Name().Value)
		}
		if err := latest.Queue.Remove(entry.ID); err != nil {
			return CellSet{}, err
		}
		removed = entry
		return latest, nil
	}); err != nil {
		return domain.QueueEntry{}, err
	}
	if _, err := (queueStarter{Cells: u.Cells, Fork: u.Fork}).StartReleased(ctx); err != nil {
		return removed, fmt.Errorf("queue entry %q is removed, but queued cells failed to start: %w", input.Issue, err)
	}
	return removed, nil
}
