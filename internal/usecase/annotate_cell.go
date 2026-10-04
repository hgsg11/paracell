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

func (u AnnotateCellUseCase) Execute(ctx context.Context, input AnnotateCellInput) (domain.CommanderCell, error) {
	var updated domain.CommanderCell
	if err := u.Cells.UpdateCells(ctx, func(cells CellSet) (CellSet, error) {
		for i, cell := range cells.Commanders {
			if cell.Matches(input.Cell) {
				if err := cell.CellGroup.SetNote(input.Note); err != nil {
					return CellSet{}, err
				}
				cells.Commanders[i] = cell
				updated = cell
				return cells, nil
			}
		}
		return CellSet{}, fmt.Errorf("cell %q not found", input.Cell)
	}); err != nil {
		return domain.CommanderCell{}, err
	}
	if err := updated.AdvanceVersion(); err != nil {
		return domain.CommanderCell{}, err
	}

	if u.WorkspaceFactory == nil {
		return updated, nil
	}
	session, err := u.WorkspaceFactory.Workspace(updated.ResourceDrivers().Workspace)
	if err != nil {
		return updated, err
	}
	if err := session.UpdateStatusLabel(ctx, updated.WorkspaceResource()); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return updated, nil
		}
		return updated, fmt.Errorf("cell note was saved, but tmux status label update failed: %w", err)
	}
	return updated, nil
}
