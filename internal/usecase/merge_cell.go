package usecase

import (
	"context"
	"fmt"

	"github.com/hgsg11/paracell/internal/domain"
)

type MergeCellInput struct {
	Cell string
	// Method is how the open pull requests are merged; empty means MergeCommit.
	Method MergeMethod
}

// MergeCellUseCase merges the pull requests of a Cell in review on the
// provider. A person decides to merge; Paracell carries it out.
type MergeCellUseCase struct {
	Config        ConfigPort
	Cells         CellPort
	ReviewFactory ReviewProviderFactory
	// Fork starts the queued Cells released by this one.
	Fork ForkCellUseCase
}

// Execute merges every pull request of the Cell that is not merged yet. Once
// all are merged, the Cell becomes merged, its Review and QueueEntry are
// removed, and the queued Cells it released start. Running it again after a
// partial failure merges only the rest. A failed merge leaves the state as it was.
// It fails with ErrNoReview when the Cell has no Review.
func (u MergeCellUseCase) Execute(ctx context.Context, input MergeCellInput) (domain.CommanderCell, error) {
	set, err := u.Cells.LoadCells(ctx)
	if err != nil {
		return domain.CommanderCell{}, err
	}
	_, group, err := set.resolve(input.Cell)
	if err != nil {
		return domain.CommanderCell{}, err
	}
	review, ok := set.review(group.ID)
	if !ok {
		return domain.CommanderCell{}, fmt.Errorf("cell %s: %w", group.Name().Value, ErrNoReview)
	}
	port, err := reviewPort(ctx, u.Config, u.ReviewFactory)
	if err != nil {
		return domain.CommanderCell{}, err
	}
	pullRequests, err := pullRequestSources(review, reviewSources(group, set.Targets))
	if err != nil {
		return domain.CommanderCell{}, err
	}
	reports, err := fetchPullRequests(ctx, port, pullRequests)
	if err != nil {
		return domain.CommanderCell{}, err
	}
	method := input.Method
	if method == "" {
		method = MergeCommit
	}
	if err := mergeOpenPullRequests(ctx, port, pullRequests, reports, method); err != nil {
		return domain.CommanderCell{}, err
	}
	return finishCommander(ctx, u.Cells, u.Fork, group.ID, (*domain.CommanderCell).MarkMerged)
}
