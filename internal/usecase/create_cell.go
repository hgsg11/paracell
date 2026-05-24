package usecase

import (
	"context"
	"fmt"

	"github.com/shige1114/paradev/internal/domain"
)

type CreateCellInput struct {
	Issue    string
	Template string
}

type CreateCellUseCase struct {
	Config           ConfigPort
	State            CellStatePort
	SourceFactory    SourceProviderFactory
	Files            FilePort
	ContainerFactory ContainerProviderFactory
	SessionFactory   SessionProviderFactory
	IDs              IDGenerator
}

func (u CreateCellUseCase) Execute(ctx context.Context, input CreateCellInput) (domain.Cell, error) {
	cfg, err := u.Config.Load(ctx, &domain.TemplateVars{
		Issue: input.Issue,
		Name:  input.Issue,
	})
	if err != nil {
		return domain.Cell{}, err
	}
	template, ok := cfg.Templates[input.Template]
	if !ok {
		return domain.Cell{}, fmt.Errorf("template %q not found", input.Template)
	}
	existing, err := u.State.LoadCells(ctx)
	if err != nil {
		return domain.Cell{}, err
	}
	name := input.Issue
	if err := (domain.CellUniquenessChecker{}).EnsureUnique(existing, input.Issue, name); err != nil {
		return domain.Cell{}, err
	}
	cell, err := domain.NewCellFactory().NewCell(u.IDs.NewID(), input.Issue, template, cfg.Project.Name)
	if err != nil {
		return domain.Cell{}, err
	}
	source, err := u.SourceFactory.Source(cfg.Providers)
	if err != nil {
		return domain.Cell{}, err
	}
	containers, err := u.ContainerFactory.Container(cfg.Providers)
	if err != nil {
		return domain.Cell{}, err
	}
	session, err := u.SessionFactory.Session(cfg.Providers)
	if err != nil {
		return domain.Cell{}, err
	}
	if err := source.CreateSource(ctx, cell); err != nil {
		return domain.Cell{}, err
	}
	if u.Files != nil {
		if err := u.Files.CopyFiles(ctx, cell, template); err != nil {
			return domain.Cell{}, err
		}
	}
	if err := containers.CreateContainers(ctx, cell, template); err != nil {
		return domain.Cell{}, err
	}
	if err := session.CreateSession(ctx, cell); err != nil {
		return domain.Cell{}, err
	}
	existing = append(existing, cell)
	if err := u.State.SaveCells(ctx, existing); err != nil {
		return domain.Cell{}, err
	}
	return cell, nil
}
