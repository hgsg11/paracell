package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hgsg11/paracell/internal/domain"
)

// ErrNoReview reports that the Cell has no Review, so its pull requests are not
// followed by Paracell.
var ErrNoReview = errors.New("cell has no review")

// reviewSource is one worktree of a Cell and the TargetCell its pull request
// belongs to. TargetCells sharing a source path share one worktree and branch,
// so the first of them owns the pull request.
type reviewSource struct {
	TargetCellID string
	TargetName   string
	Resource     domain.SourceResource
}

func reviewSources(group domain.CellGroup, targets []domain.TargetCell) []reviewSource {
	members, _ := domain.SelectCellGroupMembersService(group.ID, targets, nil)
	var sources []reviewSource
	for _, resource := range domain.BuildSourceResourcesService(group, members) {
		for _, target := range members {
			if target.Source != nil && target.Source.Path == resource.RepositoryPath {
				sources = append(sources, reviewSource{TargetCellID: target.ID, TargetName: target.Name, Resource: resource})
				break
			}
		}
	}
	return sources
}

// pullRequestSource is one pull request of a Review and the worktree it comes from.
type pullRequestSource struct {
	reviewSource
	Number string
}

// pullRequestSources pairs every pull request of the Review with its worktree.
func pullRequestSources(review domain.Review, sources []reviewSource) ([]pullRequestSource, error) {
	paired := make([]pullRequestSource, 0, len(review.PullRequests))
	for _, pullRequest := range review.PullRequests {
		found := false
		for _, source := range sources {
			if source.TargetCellID == pullRequest.TargetCellID {
				paired = append(paired, pullRequestSource{reviewSource: source, Number: pullRequest.Number})
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("pull request #%s refers to target cell %q without a source", pullRequest.Number, pullRequest.TargetCellID)
		}
	}
	return paired, nil
}

// reviewPort loads the configured review provider. It fails with ErrNoReview
// unless providers.review names a provider.
func reviewPort(ctx context.Context, config ConfigPort, factory ReviewProviderFactory) (ReviewPort, error) {
	templates, err := config.Load(ctx)
	if err != nil {
		return nil, err
	}
	if !templates.ReviewDriverType.Enabled() {
		return nil, fmt.Errorf("providers.review is not configured: %w", ErrNoReview)
	}
	return factory.Review(templates.ReviewDriverType)
}

// fetchPullRequests asks the provider for every pull request of the Review.
func fetchPullRequests(ctx context.Context, port ReviewPort, pullRequests []pullRequestSource) ([]PullRequestReport, error) {
	reports := make([]PullRequestReport, 0, len(pullRequests))
	for _, pullRequest := range pullRequests {
		report, err := port.PullRequest(ctx, pullRequest.Resource, pullRequest.Number)
		if err != nil {
			return nil, fmt.Errorf("pull request #%s: %w", pullRequest.Number, err)
		}
		reports = append(reports, report)
	}
	return reports, nil
}

func pullRequestStatuses(reports []PullRequestReport) []domain.PullRequestStatus {
	statuses := make([]domain.PullRequestStatus, 0, len(reports))
	for _, report := range reports {
		statuses = append(statuses, report.Status)
	}
	return statuses
}

// mergeOpenPullRequests merges the pull requests that are still open and leaves
// the merged ones alone. A closed pull request stops it before any merge.
func mergeOpenPullRequests(ctx context.Context, port ReviewPort, pullRequests []pullRequestSource, reports []PullRequestReport, method MergeMethod) error {
	for i, report := range reports {
		if report.Status.State == domain.PullRequestClosed {
			return fmt.Errorf("pull request #%s is closed without a merge; choose paracell merged or paracell abandon", pullRequests[i].Number)
		}
	}
	for i, report := range reports {
		if report.Status.State != domain.PullRequestOpen {
			continue
		}
		if err := port.MergePullRequest(ctx, pullRequests[i].Resource, pullRequests[i].Number, method); err != nil {
			return fmt.Errorf("merge pull request #%s: %w", pullRequests[i].Number, err)
		}
	}
	return nil
}

func formatPullRequestNumbers(pullRequests []pullRequestSource) string {
	numbers := make([]string, 0, len(pullRequests))
	for _, pullRequest := range pullRequests {
		numbers = append(numbers, "#"+pullRequest.Number)
	}
	return strings.Join(numbers, ", ")
}
