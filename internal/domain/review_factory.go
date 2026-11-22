package domain

import (
	"fmt"
	"time"
)

// RestoredPullRequestRef is the raw persisted values of one PullRequestRef.
type RestoredPullRequestRef struct {
	TargetCellID string
	Number       string
}

// ReviewFactory rebuilds a persisted Review from raw values.
type ReviewFactory struct{}

func (ReviewFactory) Restore(id string, cellGroupID string, pullRequests []RestoredPullRequestRef, handledUntil time.Time) (Review, error) {
	refs := make([]PullRequestRef, 0, len(pullRequests))
	for _, stored := range pullRequests {
		refs = append(refs, PullRequestRef{TargetCellID: stored.TargetCellID, Number: stored.Number})
	}
	review, err := NewReview(id, cellGroupID, refs)
	if err != nil {
		return Review{}, fmt.Errorf("restore review %q: %w", id, err)
	}
	if err := review.Acknowledge(handledUntil); err != nil {
		return Review{}, fmt.Errorf("restore review %q: %w", id, err)
	}
	return review, nil
}
