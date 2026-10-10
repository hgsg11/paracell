package usecase

import (
	"context"
	"fmt"

	"github.com/hgsg11/paracell/internal/domain"
)

type EnterCellInput struct {
	Cell string
}

type EnterCellUseCase struct {
	Cells            CellPort
	WorkspaceFactory WorkspaceProviderFactory
}

func (u EnterCellUseCase) Execute(ctx context.Context, input EnterCellInput) (domain.CommanderCell, error) {
	cells, err := u.Cells.LoadCells(ctx)
	if err != nil {
		return domain.CommanderCell{}, err
	}
	cell, ok := cells.FindCommander(input.Cell)
	if !ok {
		return domain.CommanderCell{}, fmt.Errorf("cell %q not found", input.Cell)
	}
	group, ok := cells.CellGroup(cell.CellGroupID)
	if !ok {
		return domain.CommanderCell{}, fmt.Errorf("CellGroup %q not found", cell.CellGroupID)
	}
	workspace, err := u.WorkspaceFactory.Workspace(group.ResourceDrivers(cell.Workspace.Driver).Workspace)
	if err != nil {
		return domain.CommanderCell{}, err
	}
	if err := workspace.EnterWorkspace(ctx, group.WorkspaceResource(cell.Workspace)); err != nil {
		return domain.CommanderCell{}, err
	}
	return cell, nil
}
