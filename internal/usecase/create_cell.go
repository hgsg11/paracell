package usecase

import (
	"context"

	"github.com/hgsg11/paracell/internal/domain"
)

type ForkCellInput struct {
	Issue    string
	Template string
	Command  string
	Note     *string
}

type ForkCellUseCase struct {
	Config           ConfigPort
	Cells            CellPort
	SourceFactory    SourceProviderFactory
	ContainerFactory ContainerProviderFactory
	SessionFactory   SessionProviderFactory
	IDs              IDGenerator
}

func (u ForkCellUseCase) Execute(ctx context.Context, input ForkCellInput) (*domain.Cell, error) {
	cfg, err := u.Config.Load(ctx)
	if err != nil {
		return nil, err
	}
	resolved, err := cfg.Resolve(input.Template, domain.NewTemplateVars(input.Issue, cfg.ProjectName, input.Command))
	if err != nil {
		return nil, err
	}
	cell, err := domain.CreateCellService(ctx, u.IDs.NewID(), input.Issue, cfg.ProjectName, input.Template, resolved, cfg.SourceDriverType, cfg.ContainerDriverType, cfg.SessionDriverType, cfg.NotificationDriverType, input.Note, u.Cells)
	if err != nil {
		return nil, err
	}
	sourcePort, err := u.SourceFactory.Source(cfg.SourceDriverType)
	if err != nil {
		return nil, err
	}
	containerPort, err := u.ContainerFactory.Container(cfg.ContainerDriverType)
	if err != nil {
		return nil, err
	}
	sessionPort, err := u.SessionFactory.Session(cfg.SessionDriverType)
	if err != nil {
		return nil, err
	}
	if err := u.Cells.CreateCell(ctx, cell); err != nil {
		return nil, err
	}

	if err := domain.CreateSourcesService(ctx, &cell, sourcePort, u.Cells); err != nil {
		return nil, err
	}
	if err := domain.CreateContainersService(ctx, &cell, containerPort, u.Cells); err != nil {
		return nil, err
	}
	if err := domain.CreateSessionService(ctx, &cell, sessionPort, u.Cells); err != nil {
		return nil, err
	}
	return &cell, nil
}
