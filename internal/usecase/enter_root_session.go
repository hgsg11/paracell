package usecase

import "context"

type EnterRootSessionUseCase struct {
	Config         ConfigPort
	SessionFactory SessionProviderFactory
}

func (u EnterRootSessionUseCase) Execute(ctx context.Context) error {
	cfg, err := u.Config.Load(ctx)
	if err != nil {
		return err
	}
	session, err := u.SessionFactory.Session(cfg.SessionDriverType)
	if err != nil {
		return err
	}
	return session.EnterRootSession(ctx, cfg.ProjectName)
}
