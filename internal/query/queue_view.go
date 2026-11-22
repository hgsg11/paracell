package query

import "context"

const (
	QueueStateWaiting = "waiting"
	QueueStateRunning = "running"

	PlannedPathModify = "modify"
	PlannedPathCreate = "create"
)

// QueueView is one unfinished QueueEntry.
type QueueView struct {
	Issue string
	Note  string
	// State is "waiting" or "running".
	State string
	// BlockedBy lists the issues of the earlier entries this one overlaps.
	BlockedBy []string
	Modify    []string
	Create    []string
}

// PlannedPathView is one path declared by an unfinished QueueEntry.
type PlannedPathView struct {
	// Kind is "modify" or "create".
	Kind  string
	Path  string
	Issue string
	Note  string
}

// QueueViewQuery reads the Queue without changing it.
type QueueViewQuery interface {
	// ListQueueViews lists the unfinished entries in enqueue order.
	ListQueueViews(ctx context.Context) ([]QueueView, error)
	// ListPlannedPaths lists the declared paths that start with prefix, sorted
	// by path and then enqueue order.
	ListPlannedPaths(ctx context.Context, prefix string) ([]PlannedPathView, error)
}
