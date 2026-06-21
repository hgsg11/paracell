package usecase

import (
	"context"
)

type EnterRootSessionUseCase struct {
	Config         ConfigPort
	SessionFactory SessionProviderFactory
}

func (u EnterRootSessionUseCase) Execute(ctx context.Context) error {
	cfg, err := u.Config.Load(ctx, nil)
	if err != nil {
		return err
	}
	session, err := u.SessionFactory.Session(cfg.Providers)
	if err != nil {
		return err
	}
	return session.EnterRootSession(ctx, cfg.Project)
}
