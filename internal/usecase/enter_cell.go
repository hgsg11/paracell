package usecase

import (
	"context"

	"github.com/hgsg11/paracell/internal/domain"
)

type EnterCellInput struct {
	Cell domain.Cell
}

type EnterCellUseCase struct {
	SessionFactory SessionProviderFactory
}

func (u EnterCellUseCase) Execute(ctx context.Context, input EnterCellInput) (domain.Cell, error) {
	session, err := u.SessionFactory.Session(input.Cell.ResourceDrivers().Session)
	if err != nil {
		return domain.Cell{}, err
	}
	name, cellName, project, label, windows := input.Cell.SessionPreparation()
	if err := session.EnterSession(ctx, name, cellName, project, label, windows); err != nil {
		return domain.Cell{}, err
	}
	return input.Cell, nil
}
