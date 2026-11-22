// Package query holds read models for displaying Cells. It does not depend on
// the domain, use case, or adapter packages.
package query

import "context"

// CellView is one row of the Cell list shown by `paracell ls` and the TUI.
type CellView struct {
	// Name is the Cell name.
	Name string
	// Label is the CellGroup note, or Name when the note is empty.
	Label    string
	Template string
	// Status is "pending" or "ready".
	Status string
	// Progress is "in_progress", "in_review", "merged", or "abandoned".
	Progress string
	// CreationStatus is "creating", "failed", or "ready".
	CreationStatus string
	// FailedStage and LastError are set only when the creation failed.
	FailedStage string
	LastError   string
	// PullRequests is the provider's numbers of the pull requests in review,
	// empty when the Cell has no Review.
	PullRequests []string
	// NeedsDecision is true when the pull requests are partly merged and partly
	// closed, so a person chooses merged or abandoned. The provider tells it,
	// not the state, so the state query leaves it false.
	NeedsDecision bool
}

// CellViewQuery lists Cells in their persisted order without restoring domain models.
type CellViewQuery interface {
	ListCellViews(ctx context.Context) ([]CellView, error)
}
