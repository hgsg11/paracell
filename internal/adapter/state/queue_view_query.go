package state

import (
	"context"
	"sort"
	"strings"

	"github.com/hgsg11/paracell/internal/domain"
	"github.com/hgsg11/paracell/internal/query"
	"github.com/hgsg11/paracell/internal/usecase"
)

var _ query.QueueViewQuery = SQLiteCellAdapter{}

func (a SQLiteCellAdapter) ListQueueViews(ctx context.Context) ([]query.QueueView, error) {
	set, err := a.loadForQuery(ctx)
	if err != nil {
		return nil, err
	}
	views := make([]query.QueueView, 0, len(set.Queue.Entries))
	for _, entry := range set.Queue.Entries {
		view := query.QueueView{
			Issue: entry.Issue, Note: entry.Request.Note, State: query.QueueStateWaiting,
			Modify: pathValues(entry.Plan.Modify), Create: pathValues(entry.Plan.Create),
		}
		if _, running := domain.FindQueueEntryCellGroupService(set.Groups, entry.ID); running {
			view.State = query.QueueStateRunning
		} else {
			for _, blocker := range set.Queue.Blockers(entry.ID) {
				view.BlockedBy = append(view.BlockedBy, blocker.Issue)
			}
		}
		views = append(views, view)
	}
	return views, nil
}

func (a SQLiteCellAdapter) ListPlannedPaths(ctx context.Context, prefix string) ([]query.PlannedPathView, error) {
	set, err := a.loadForQuery(ctx)
	if err != nil {
		return nil, err
	}
	var views []query.PlannedPathView
	for _, entry := range set.Queue.Entries {
		for _, kind := range []struct {
			name  string
			paths []domain.WorkspacePath
		}{{query.PlannedPathModify, entry.Plan.Modify}, {query.PlannedPathCreate, entry.Plan.Create}} {
			for _, path := range kind.paths {
				if strings.HasPrefix(path.Value, prefix) {
					views = append(views, query.PlannedPathView{Kind: kind.name, Path: path.Value, Issue: entry.Issue, Note: entry.Request.Note})
				}
			}
		}
	}
	// Entries are in enqueue order, so a stable sort keeps it within a path.
	sort.SliceStable(views, func(i, j int) bool { return views[i].Path < views[j].Path })
	return views, nil
}

// loadForQuery restores the Queue and the CellGroups so that the read model
// reuses the domain rules for waiting, running, and blockers.
func (a SQLiteCellAdapter) loadForQuery(ctx context.Context) (usecase.CellSet, error) {
	db, err := a.open(ctx)
	if err != nil {
		return usecase.CellSet{}, err
	}
	defer db.Close()
	return loadCells(ctx, db)
}

func pathValues(paths []domain.WorkspacePath) []string {
	values := make([]string, 0, len(paths))
	for _, path := range paths {
		values = append(values, path.Value)
	}
	return values
}
