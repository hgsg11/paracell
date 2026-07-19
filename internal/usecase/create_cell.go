package usecase

import (
	"context"
	"fmt"

	"github.com/hgsg11/paracell/internal/domain"
)

type ForkCellInput struct {
	Issue    string
	Template string
	Input    string
}

type ForkCellUseCase struct {
	Config           ConfigPort
	State            CellStatePort
	CellFactory      CellFactory
	SourceFactory    SourceProviderFactory
	Files            FilePort
	ContainerFactory ContainerProviderFactory
	SessionFactory   SessionProviderFactory
	IDs              IDGenerator
}

func (u ForkCellUseCase) Execute(ctx context.Context, input ForkCellInput) (domain.Cell, error) {
	cfg, err := u.Config.Load(ctx, &domain.TemplateVars{
		Issue: input.Issue,
		Name:  input.Issue,
		Input: input.Input,
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
	cell, err := u.CellFactory.NewCell(u.IDs.NewID(), input.Issue, template, cfg.Project.Name)
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
		if err := u.Files.CopyFiles(ctx, cell, templateWithInitFiles(template)); err != nil {
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

func templateWithInitFiles(template domain.Template) domain.Template {
	merged := template
	files := append([]string(nil), template.Files...)
	seen := make(map[string]struct{}, len(files))
	for _, file := range files {
		seen[file] = struct{}{}
	}
	for _, service := range template.Containers.Services {
		if service.Database == nil {
			continue
		}
		for _, file := range service.Database.InitFiles {
			if _, ok := seen[file]; ok {
				continue
			}
			files = append(files, file)
			seen[file] = struct{}{}
		}
	}
	merged.Files = files
	return merged
}
