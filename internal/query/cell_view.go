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
	Done   bool
	// CreationStatus is "creating", "failed", or "ready".
	CreationStatus string
	// FailedStage and LastError are set only when the creation failed.
	FailedStage string
	LastError   string
}

// CellViewQuery lists Cells in their persisted order without restoring domain models.
type CellViewQuery interface {
	ListCellViews(ctx context.Context) ([]CellView, error)
}
