package usecase

import (
	"context"

	"github.com/hgsg11/paracell/internal/domain"
)

type EnterCellInput struct {
	Cell domain.CommanderCell
}

type EnterCellUseCase struct {
	WorkspaceFactory WorkspaceProviderFactory
}

func (u EnterCellUseCase) Execute(ctx context.Context, input EnterCellInput) (domain.CommanderCell, error) {
	workspace, err := u.WorkspaceFactory.Workspace(input.Cell.ResourceDrivers().Workspace)
	if err != nil {
		return domain.CommanderCell{}, err
	}
	if err := workspace.EnterWorkspace(ctx, input.Cell.WorkspaceResource()); err != nil {
		return domain.CommanderCell{}, err
	}
	return input.Cell, nil
}
