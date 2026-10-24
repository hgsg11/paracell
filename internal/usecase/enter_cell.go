package usecase

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/hgsg11/paracell/internal/domain"
)

type EnterCellInput struct {
	Cell string
}

// EnterCellUseCase prepares the session of a Cell, named by the user, and returns
// the command that moves this terminal into it.
type EnterCellUseCase struct {
	Cells            CellPort
	WorkspaceFactory WorkspaceProviderFactory
}

func (u EnterCellUseCase) Execute(ctx context.Context, input EnterCellInput) (*exec.Cmd, error) {
	cells, err := u.Cells.LoadCells(ctx)
	if err != nil {
		return nil, err
	}
	commander, group, err := cells.resolve(input.Cell)
	if err != nil {
		return nil, err
	}
	driver := group.ResourceDrivers(commander.Workspace.Driver).Workspace
	if driver != domain.Tmux {
		return nil, fmt.Errorf("unsupported session driver %q", driver)
	}
	workspace, err := u.WorkspaceFactory.Workspace(driver)
	if err != nil {
		return nil, err
	}
	return workspace.EnterCommand(ctx, group.WorkspaceResource(commander.Workspace))
}
