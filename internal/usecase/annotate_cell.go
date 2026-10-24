package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/hgsg11/paracell/internal/domain"
)

type AnnotateCellInput struct {
	Cell string
	Note string
}

type AnnotateCellUseCase struct {
	Cells            CellPort
	WorkspaceFactory WorkspaceProviderFactory
}

// Execute sets the note on the CellGroup of the resolved Cell and refreshes the
// tmux status label. It returns the CellGroup as saved.
func (u AnnotateCellUseCase) Execute(ctx context.Context, input AnnotateCellInput) (domain.CellGroup, error) {
	var updated domain.CellGroup
	var commander domain.CommanderCell
	if err := u.Cells.UpdateCells(ctx, func(cells CellSet) (CellSet, error) {
		found, group, err := cells.resolve(input.Cell)
		if err != nil {
			return CellSet{}, err
		}
		if err := group.SetNote(input.Note); err != nil {
			return CellSet{}, err
		}
		if err := cells.replaceGroup(group); err != nil {
			return CellSet{}, err
		}
		updated, commander = group, found
		return cells, nil
	}); err != nil {
		return domain.CellGroup{}, err
	}

	if u.WorkspaceFactory == nil {
		return updated, nil
	}
	session, err := u.WorkspaceFactory.Workspace(updated.ResourceDrivers(commander.Workspace.Driver).Workspace)
	if err != nil {
		return updated, err
	}
	if err := session.UpdateStatusLabel(ctx, updated.WorkspaceResource(commander.Workspace)); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return updated, nil
		}
		return updated, fmt.Errorf("cell note was saved, but tmux status label update failed: %w", err)
	}
	return updated, nil
}
