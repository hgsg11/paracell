package usecase

import "context"

type ExitSessionUseCase struct {
	Config         ConfigPort
	SessionFactory SessionProviderFactory
}

func (u ExitSessionUseCase) Execute(ctx context.Context) error {
	cfg, err := u.Config.Load(ctx, nil)
	if err != nil {
		return err
	}
	session, err := u.SessionFactory.Session(cfg.Providers)
	if err != nil {
		return err
	}
	return session.ExitSession(ctx)
}
