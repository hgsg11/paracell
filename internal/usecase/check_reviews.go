package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/hgsg11/paracell/internal/domain"
)

// CheckReviewsUseCase asks the review provider where the pull requests of
// every Cell in review stand, so that a person can see the Cells that need a
// decision.
type CheckReviewsUseCase struct {
	Config        ConfigPort
	Cells         CellPort
	ReviewFactory ReviewProviderFactory
}

// Execute returns the outcome of each Cell with a Review by Cell name. It is
// empty without a review provider. A Cell whose pull requests cannot be read
// is left out and reported in the joined error.
func (u CheckReviewsUseCase) Execute(ctx context.Context) (map[string]domain.ReviewOutcome, error) {
	set, err := u.Cells.LoadCells(ctx)
	if err != nil || len(set.Reviews) == 0 {
		return nil, err
	}
	port, err := reviewPort(ctx, u.Config, u.ReviewFactory)
	if errors.Is(err, ErrNoReview) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	outcomes := make(map[string]domain.ReviewOutcome, len(set.Reviews))
	var checkErr error
	for _, review := range set.Reviews {
		group, ok := set.group(review.CellGroupID)
		if !ok {
			continue
		}
		name := group.Name().Value
		pullRequests, err := pullRequestSources(review, reviewSources(group, set.Targets))
		if err != nil {
			checkErr = errors.Join(checkErr, fmt.Errorf("cell %s: %w", name, err))
			continue
		}
		reports, err := fetchPullRequests(ctx, port, pullRequests)
		if err != nil {
			checkErr = errors.Join(checkErr, fmt.Errorf("cell %s: %w", name, err))
			continue
		}
		outcomes[name] = domain.DecideReviewOutcome(pullRequestStatuses(reports))
	}
	return outcomes, checkErr
}
