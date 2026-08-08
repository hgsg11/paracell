package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/hgsg11/paracell/internal/domain"
)

type ForkCellInput struct {
	Issue    string
	Template string
	Command  string
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
		Issue:   input.Issue,
		Name:    input.Issue,
		Command: input.Command,
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
	cell, err := u.CellFactory.NewCell(u.IDs.NewID(), input.Issue, template, cfg.Project.Name)
	if err != nil {
		return domain.Cell{}, err
	}
	if err := (domain.CellUniquenessChecker{}).EnsureUnique(existing, input.Issue, cell.Name); err != nil {
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
	progress := forkProgress{}
	progress.sourceStarted = true
	creation, err := source.CreateSource(ctx, cell)
	progress.sourceCreation = creation
	if err != nil {
		return domain.Cell{}, rollbackFork(ctx, err, cell, source, containers, session, progress)
	}
	if u.Files != nil {
		if err := u.Files.CopyFiles(ctx, cell, templateWithInitFiles(template)); err != nil {
			return domain.Cell{}, rollbackFork(ctx, err, cell, source, containers, session, progress)
		}
	}
	progress.containersStarted = true
	if err := containers.CreateContainers(ctx, cell, template); err != nil {
		return domain.Cell{}, rollbackFork(ctx, err, cell, source, containers, session, progress)
	}
	progress.sessionStarted = true
	if err := session.CreateSession(ctx, cell); err != nil {
		return domain.Cell{}, rollbackFork(ctx, err, cell, source, containers, session, progress)
	}
	if err := u.State.UpdateCells(ctx, func(latest []domain.Cell) ([]domain.Cell, error) {
		if err := (domain.CellUniquenessChecker{}).EnsureUnique(latest, input.Issue, cell.Name); err != nil {
			return nil, err
		}
		return append(latest, cell), nil
	}); err != nil {
		return domain.Cell{}, rollbackFork(ctx, err, cell, source, containers, session, progress)
	}
	return cell, nil
}

type forkProgress struct {
	sourceStarted     bool
	sourceCreation    SourceCreation
	containersStarted bool
	sessionStarted    bool
}

func rollbackFork(
	ctx context.Context,
	createErr error,
	cell domain.Cell,
	source SourcePort,
	containers ContainerPort,
	session SessionPort,
	progress forkProgress,
) error {
	rollbackCtx := context.WithoutCancel(ctx)
	joined := createErr
	if progress.sessionStarted {
		if err := ignoreNotFound(session.CleanSession(rollbackCtx, cell)); err != nil {
			joined = errors.Join(joined, fmt.Errorf("rollback session: %w", err))
		}
	}
	if progress.containersStarted {
		if err := ignoreNotFound(containers.CleanContainers(rollbackCtx, cell)); err != nil {
			joined = errors.Join(joined, fmt.Errorf("rollback containers: %w", err))
		}
	}
	if progress.sourceStarted {
		if err := ignoreNotFound(source.RollbackSource(rollbackCtx, cell, progress.sourceCreation)); err != nil {
			joined = errors.Join(joined, fmt.Errorf("rollback source: %w", err))
		}
	}
	return joined
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
