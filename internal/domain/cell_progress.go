package domain

import "fmt"

// CellProgress is how far the implementation of a Cell has come. It is a
// separate axis from CellStatus, which tells whether the AI agent is working.
type CellProgress string

const (
	InProgress CellProgress = "in_progress"
	InReview   CellProgress = "in_review"
	Merged     CellProgress = "merged"
	Abandoned  CellProgress = "abandoned"
)

func NewCellProgress(value string) (CellProgress, error) {
	progress := CellProgress(value)
	switch progress {
	case InProgress, InReview, Merged, Abandoned:
		return progress, nil
	default:
		return progress, fmt.Errorf("unsupported progress %q", progress)
	}
}

// IsTerminal reports whether the progress can no longer change. Merged and
// abandoned Cells stay as they are.
func (p CellProgress) IsTerminal() bool {
	return p == Merged || p == Abandoned
}

// transitionTo returns next when the move from p to next is allowed. Moving to
// the same progress is not allowed.
func (p CellProgress) transitionTo(next CellProgress) (CellProgress, error) {
	allowed := false
	switch p {
	case InProgress:
		allowed = next == InReview || next == Merged || next == Abandoned
	case InReview:
		allowed = next == InProgress || next == Merged || next == Abandoned
	}
	if !allowed {
		return p, fmt.Errorf("cell cannot move from %s to %s", p, next)
	}
	return next, nil
}
