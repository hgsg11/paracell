package domain

import (
	"context"
	"fmt"
)

type SessionCreationPort interface {
	CreateSession(ctx context.Context, name string, cellName string, firstWindow string, workingDirectory string) error
	CreateWindow(ctx context.Context, session string, window string, workingDirectory string) error
	SendWindowCommand(ctx context.Context, session string, window string, command string) error
	ConfigureSession(ctx context.Context, name string, cellName string, project string, label string, windowNames []string) error
}

type SessionCellSavePort interface {
	SaveCell(ctx context.Context, cell Cell) error
}

func CreateSessionService(ctx context.Context, cell *Cell, port SessionCreationPort, cells SessionCellSavePort) error {
	fail := func(err error) error {
		if stateErr := cell.FailCreation(CreationStageSession, err); stateErr != nil {
			return fmt.Errorf("%w; record session creation failure: %v", err, stateErr)
		}
		if saveErr := cells.SaveCell(ctx, *cell); saveErr != nil {
			return fmt.Errorf("%w; save session creation failure: %v", err, saveErr)
		}
		return err
	}
	if err := cell.SetCreationStage(CreationStageSession); err != nil {
		return err
	}
	if err := cells.SaveCell(ctx, *cell); err != nil {
		return err
	}
	name, cellName, project, label, windowNames := cell.SessionPreparation()
	workingDirectory := cell.WorkingDirectory()
	firstWindow := ""
	if len(cell.Session.Windows) > 0 {
		firstWindow = cell.Session.Windows[0].Name
	}
	if err := port.CreateSession(ctx, name, cellName, firstWindow, workingDirectory); err != nil {
		return fail(err)
	}
	for index, window := range cell.Session.Windows {
		if index > 0 {
			if err := port.CreateWindow(ctx, name, window.Name, workingDirectory); err != nil {
				return fail(err)
			}
		}
		if window.Command != "" {
			if err := port.SendWindowCommand(ctx, name, window.Name, window.Command); err != nil {
				return fail(err)
			}
		}
	}
	if err := port.ConfigureSession(ctx, name, cellName, project, label, windowNames); err != nil {
		return fail(err)
	}
	if err := cell.FinishCreation(); err != nil {
		return err
	}
	return cells.SaveCell(ctx, *cell)
}
