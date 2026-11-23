package state

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/hgsg11/paracell/internal/domain"
)

// reviewSchema stores one row per Review and one row per pull request of it.
// A Review row exists only while its Cell is in review.
var reviewSchema = []string{
	`CREATE TABLE IF NOT EXISTS reviews (
		id TEXT PRIMARY KEY,
		cell_group_id TEXT NOT NULL UNIQUE,
		handled_until TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS review_pull_requests (
		review_id TEXT NOT NULL REFERENCES reviews(id) ON DELETE CASCADE,
		position INTEGER NOT NULL,
		target_cell_id TEXT NOT NULL,
		number TEXT NOT NULL,
		PRIMARY KEY (review_id, position),
		UNIQUE (review_id, target_cell_id)
	)`,
}

// storedReview is one reviews row with its pull request rows, before the
// domain validates it.
type storedReview struct {
	ID           string
	CellGroupID  string
	HandledUntil string
	PullRequests []domain.RestoredPullRequestRef
}

// queryReviews reads every Review row with its pull requests in position order.
func queryReviews(ctx context.Context, queryer stateQueryer) ([]storedReview, error) {
	rows, err := queryer.QueryContext(ctx, "SELECT id, cell_group_id, handled_until FROM reviews ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("query reviews: %w", err)
	}
	var stored []storedReview
	index := map[string]int{}
	for rows.Next() {
		var review storedReview
		if err := rows.Scan(&review.ID, &review.CellGroupID, &review.HandledUntil); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan review: %w", err)
		}
		index[review.ID] = len(stored)
		stored = append(stored, review)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate reviews: %w", err)
	}
	rows.Close()
	pullRequests, err := queryer.QueryContext(ctx, "SELECT review_id, target_cell_id, number FROM review_pull_requests ORDER BY review_id, position")
	if err != nil {
		return nil, fmt.Errorf("query review pull requests: %w", err)
	}
	defer pullRequests.Close()
	for pullRequests.Next() {
		var reviewID string
		var ref domain.RestoredPullRequestRef
		if err := pullRequests.Scan(&reviewID, &ref.TargetCellID, &ref.Number); err != nil {
			return nil, fmt.Errorf("scan review pull request: %w", err)
		}
		i, ok := index[reviewID]
		if !ok {
			return nil, fmt.Errorf("pull request #%s refers to unknown review %q", ref.Number, reviewID)
		}
		stored[i].PullRequests = append(stored[i].PullRequests, ref)
	}
	if err := pullRequests.Err(); err != nil {
		return nil, fmt.Errorf("iterate review pull requests: %w", err)
	}
	return stored, nil
}

// loadReviews restores every persisted Review.
func loadReviews(ctx context.Context, queryer stateQueryer) ([]domain.Review, error) {
	stored, err := queryReviews(ctx, queryer)
	if err != nil {
		return nil, err
	}
	reviews := make([]domain.Review, 0, len(stored))
	for _, row := range stored {
		handledUntil, err := time.Parse(time.RFC3339Nano, row.HandledUntil)
		if err != nil {
			return nil, fmt.Errorf("restore review %q: handled time: %w", row.ID, err)
		}
		review, err := domain.ReviewFactory{}.Restore(row.ID, row.CellGroupID, row.PullRequests, handledUntil)
		if err != nil {
			return nil, err
		}
		reviews = append(reviews, review)
	}
	return reviews, nil
}

// applyReviewChanges deletes the Reviews that are gone, inserts the new ones,
// and updates the handled time of the others. The pull requests of a Review
// never change after it is created.
func applyReviewChanges(ctx context.Context, execer stateExecer, current []domain.Review, next []domain.Review) error {
	currentByID := make(map[string]domain.Review, len(current))
	for _, review := range current {
		currentByID[review.ID] = review
	}
	nextByID := make(map[string]struct{}, len(next))
	for _, review := range next {
		nextByID[review.ID] = struct{}{}
	}
	for _, review := range current {
		if _, exists := nextByID[review.ID]; exists {
			continue
		}
		if _, err := execer.ExecContext(ctx, "DELETE FROM reviews WHERE id = ?", review.ID); err != nil {
			return fmt.Errorf("delete review %q: %w", review.ID, err)
		}
	}
	for _, review := range next {
		stored, exists := currentByID[review.ID]
		if !exists {
			if err := insertReview(ctx, execer, review); err != nil {
				return err
			}
			continue
		}
		if stored.CellGroupID != review.CellGroupID || !reflect.DeepEqual(stored.PullRequests, review.PullRequests) {
			return fmt.Errorf("review %q cannot change its Cell or pull requests", review.ID)
		}
		if stored.HandledUntil.Equal(review.HandledUntil) {
			continue
		}
		if _, err := execer.ExecContext(ctx, "UPDATE reviews SET handled_until = ? WHERE id = ?", formatHandledUntil(review.HandledUntil), review.ID); err != nil {
			return fmt.Errorf("update review %q: %w", review.ID, err)
		}
	}
	return nil
}

func insertReview(ctx context.Context, execer stateExecer, review domain.Review) error {
	if _, err := execer.ExecContext(ctx, "INSERT INTO reviews (id, cell_group_id, handled_until) VALUES (?, ?, ?)",
		review.ID, review.CellGroupID, formatHandledUntil(review.HandledUntil)); err != nil {
		return fmt.Errorf("insert review %q: %w", review.ID, err)
	}
	for position, pullRequest := range review.PullRequests {
		if _, err := execer.ExecContext(ctx, "INSERT INTO review_pull_requests (review_id, position, target_cell_id, number) VALUES (?, ?, ?, ?)",
			review.ID, position, pullRequest.TargetCellID, pullRequest.Number); err != nil {
			return fmt.Errorf("insert review %q pull request #%s: %w", review.ID, pullRequest.Number, err)
		}
	}
	return nil
}

func formatHandledUntil(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

// cloneReviews copies the Reviews and their pull request lists so that the
// update callback cannot change the loaded ones.
func cloneReviews(reviews []domain.Review) []domain.Review {
	cloned := make([]domain.Review, 0, len(reviews))
	for _, review := range reviews {
		review.PullRequests = append([]domain.PullRequestRef(nil), review.PullRequests...)
		cloned = append(cloned, review)
	}
	return cloned
}

// pullRequestNumbersByGroup reads the pull request numbers of every Review by
// CellGroup ID, without restoring the Reviews.
func pullRequestNumbersByGroup(ctx context.Context, queryer stateQueryer) (map[string][]string, error) {
	stored, err := queryReviews(ctx, queryer)
	if err != nil {
		return nil, err
	}
	numbers := make(map[string][]string, len(stored))
	for _, review := range stored {
		for _, pullRequest := range review.PullRequests {
			numbers[review.CellGroupID] = append(numbers[review.CellGroupID], pullRequest.Number)
		}
	}
	return numbers, nil
}
