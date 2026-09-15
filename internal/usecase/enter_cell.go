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
	session, err := u.SessionFactory.Session(domain.CellResourceDrivers(input.Cell).Session)
	if err != nil {
		return domain.Cell{}, err
	}
	if err := domain.EnterSession(ctx, input.Cell, session); err != nil {
		return domain.Cell{}, err
	}
	return input.Cell, nil
}
