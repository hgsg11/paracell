package usecase

import (
	"context"

	"github.com/hgsg11/paracell/internal/domain"
)

type EnterCellInput struct {
	Cell domain.Cell
}

type EnterCellUseCase struct {
	Config         ConfigPort
	SessionFactory SessionProviderFactory
}

func (u EnterCellUseCase) Execute(ctx context.Context, input EnterCellInput) (domain.Cell, error) {
	cfg, err := u.Config.Load(ctx, nil)
	if err != nil {
		return domain.Cell{}, err
	}
	session, err := u.SessionFactory.Session(cfg.Providers)
	if err != nil {
		return domain.Cell{}, err
	}
	if err := session.EnterSession(ctx, input.Cell); err != nil {
		return domain.Cell{}, err
	}
	return input.Cell, nil
}
