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
	var updatedGroup domain.CellGroup
	if err := u.Cells.UpdateCells(ctx, func(cells CellSet) (CellSet, error) {
		cell, ok := cells.FindCommander(input.Cell)
		if !ok {
			return CellSet{}, fmt.Errorf("cell %q not found", input.Cell)
		}
		for i := range cells.Groups {
			if cells.Groups[i].ID == cell.CellGroupID {
				if err := cells.Groups[i].SetNote(input.Note); err != nil {
					return CellSet{}, err
				}
				updatedGroup = cells.Groups[i]
				updated = cell
				return cells, nil
			}
		}
		return CellSet{}, fmt.Errorf("CellGroup %q not found", cell.CellGroupID)
	}); err != nil {
		return domain.CommanderCell{}, err
	}
	if err := updated.AdvanceVersion(); err != nil {
		return domain.CommanderCell{}, err
	}

	if u.WorkspaceFactory == nil {
		return updated, nil
	}
	session, err := u.WorkspaceFactory.Workspace(updatedGroup.ResourceDrivers(updated.Workspace.Driver).Workspace)
	if err != nil {
		return updated, err
	}
	if err := session.UpdateStatusLabel(ctx, updatedGroup.WorkspaceResource(updated.Workspace)); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return updated, nil
		}
		return updated, fmt.Errorf("cell note was saved, but tmux status label update failed: %w", err)
	}
	return updated, nil
}
