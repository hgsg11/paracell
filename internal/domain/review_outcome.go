package domain

// PullRequestState is the provider's state of one pull request.
type PullRequestState string

const (
	PullRequestOpen   PullRequestState = "open"
	PullRequestMerged PullRequestState = "merged"
	PullRequestClosed PullRequestState = "closed"
)

// PullRequestApproval summarizes the standing reviews of one pull request.
type PullRequestApproval string

const (
	ApprovalNone             PullRequestApproval = "none"
	ApprovalApproved         PullRequestApproval = "approved"
	ApprovalChangesRequested PullRequestApproval = "changes_requested"
)

// PullRequestStatus is what the provider reported for one pull request.
type PullRequestStatus struct {
	State    PullRequestState
	Approval PullRequestApproval
}

// ReviewOutcome is where the pull requests of a Review stand as a whole.
type ReviewOutcome string

const (
	// ReviewOpen means at least one pull request is still open.
	ReviewOpen ReviewOutcome = "open"
	// ReviewMerged means every pull request is merged.
	ReviewMerged ReviewOutcome = "merged"
	// ReviewClosed means every pull request is closed without a merge.
	ReviewClosed ReviewOutcome = "closed"
	// ReviewMixed means none is open, but some are merged and others closed.
	// A person decides whether the Cell is merged or abandoned.
	ReviewMixed ReviewOutcome = "mixed"
)

// DecideReviewOutcome derives the outcome from the pull request states.
func DecideReviewOutcome(statuses []PullRequestStatus) ReviewOutcome {
	merged, closed := 0, 0
	for _, status := range statuses {
		switch status.State {
		case PullRequestMerged:
			merged++
		case PullRequestClosed:
			closed++
		default:
			return ReviewOpen
		}
	}
	switch {
	case merged > 0 && closed > 0:
		return ReviewMixed
	case closed > 0:
		return ReviewClosed
	case merged > 0:
		return ReviewMerged
	default:
		return ReviewOpen
	}
}

// IsReviewApproved reports whether the open pull requests can be merged on
// their reviews: at least one is open, every open one is approved, and none is
// closed. Who approved is not checked.
func IsReviewApproved(statuses []PullRequestStatus) bool {
	open := 0
	for _, status := range statuses {
		switch status.State {
		case PullRequestOpen:
			if status.Approval != ApprovalApproved {
				return false
			}
			open++
		case PullRequestClosed:
			return false
		}
	}
	return open > 0
}
