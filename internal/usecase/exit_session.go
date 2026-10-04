package usecase

import "context"

type ExitWorkspaceUseCase struct {
	Config           ConfigPort
	WorkspaceFactory WorkspaceProviderFactory
}

func (u ExitWorkspaceUseCase) Execute(ctx context.Context) error {
	cfg, err := u.Config.Load(ctx)
	if err != nil {
		return err
	}
	session, err := u.WorkspaceFactory.Workspace(cfg.WorkspaceDriverType)
	if err != nil {
		return err
	}
	return session.ExitWorkspace(ctx)
}
