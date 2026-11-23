package domain

import (
	"fmt"
	"time"
)

// PullRequestRef is one pull request opened for a TargetCell. Number is the
// provider's number as a string, so that it also fits merge requests and other
// providers. The pull request state is not part of it: the provider holds it.
type PullRequestRef struct {
	TargetCellID string
	Number       string
}

func NewPullRequestRef(targetCellID string, number string) (PullRequestRef, error) {
	if targetCellID == "" || number == "" {
		return PullRequestRef{}, fmt.Errorf("pull request target cell and number are required")
	}
	return PullRequestRef{TargetCellID: targetCellID, Number: number}, nil
}

// Review is the aggregate root that exists while the pull requests of a Cell
// are in review. There is at most one per CellGroup: it is created by the first
// review request and removed when the Cell becomes merged or abandoned, so a
// finished Review is never kept. HandledUntil is the time up to which review
// events have been delivered to the agent.
type Review struct {
	ID           string
	CellGroupID  string
	PullRequests []PullRequestRef
	HandledUntil time.Time
}

// NewReview requires at least one pull request and at most one per TargetCell.
func NewReview(id string, cellGroupID string, pullRequests []PullRequestRef) (Review, error) {
	if id == "" || cellGroupID == "" {
		return Review{}, fmt.Errorf("review id and CellGroup reference are required")
	}
	if len(pullRequests) == 0 {
		return Review{}, fmt.Errorf("review requires at least one pull request")
	}
	targets := make(map[string]struct{}, len(pullRequests))
	for _, pullRequest := range pullRequests {
		validated, err := NewPullRequestRef(pullRequest.TargetCellID, pullRequest.Number)
		if err != nil {
			return Review{}, err
		}
		if _, exists := targets[validated.TargetCellID]; exists {
			return Review{}, fmt.Errorf("target cell %q has more than one pull request", validated.TargetCellID)
		}
		targets[validated.TargetCellID] = struct{}{}
	}
	return Review{ID: id, CellGroupID: cellGroupID, PullRequests: append([]PullRequestRef(nil), pullRequests...)}, nil
}

// Acknowledge records that the events up to until were delivered. It only
// moves HandledUntil forward.
func (r *Review) Acknowledge(until time.Time) error {
	if until.Before(r.HandledUntil) {
		return fmt.Errorf("review handled time cannot move back from %s to %s", r.HandledUntil.Format(time.RFC3339), until.Format(time.RFC3339))
	}
	r.HandledUntil = until
	return nil
}
