package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/hgsg11/paracell/internal/domain"
)

type RetryCellInput struct {
	Cell string
}

type RetryCellUseCase struct {
	Config           ConfigPort
	State            CellStatePort
	CellFactory      CellFactory
	SourceFactory    SourceProviderFactory
	Files            FilePort
	ContainerFactory ContainerProviderFactory
	SessionFactory   SessionProviderFactory
}

func (u RetryCellUseCase) Execute(ctx context.Context, input RetryCellInput) (domain.Cell, error) {
	cells, err := u.State.LoadCells(ctx)
	if err != nil {
		return domain.Cell{}, err
	}
	cell, ok := resolveCell(cells, input.Cell)
	if !ok {
		return domain.Cell{}, fmt.Errorf("cell %q not found", input.Cell)
	}
	if cell.CreationStatus() != domain.CreationFailed {
		return domain.Cell{}, fmt.Errorf("cell %q is %s and cannot be retried", cell.Name, cell.CreationStatus())
	}

	failValidation := func(validationErr error) (domain.Cell, error) {
		stage := cell.Creation.FailedStage
		if stage == "" {
			stage = nextCreationStage(cell)
		}
		cell.FailCreation(stage, validationErr)
		if saveErr := replaceCell(context.WithoutCancel(ctx), u.State, cell); saveErr != nil {
			return domain.Cell{}, errors.Join(validationErr, fmt.Errorf("save failed cell: %w", saveErr))
		}
		return domain.Cell{}, validationErr
	}

	cfg, err := u.Config.Load(ctx, &domain.TemplateVars{
		Issue:   cell.Issue,
		Name:    cell.Name,
		Command: cell.Creation.Command,
	})
	if err != nil {
		return failValidation(err)
	}
	template, ok := cfg.Templates[cell.Template]
	if !ok {
		return failValidation(fmt.Errorf("template %q not found", cell.Template))
	}
	rendered, err := u.CellFactory.NewCell(cell.ID, cell.Issue, template, cfg.Project.Name)
	if err != nil {
		return failValidation(err)
	}
	stored := cell
	cell = refreshRetryCell(cell, rendered)
	source, err := u.SourceFactory.Source(cfg.Providers)
	if err != nil {
		return failValidation(err)
	}
	containers, err := u.ContainerFactory.Container(cfg.Providers)
	if err != nil {
		return failValidation(err)
	}
	session, err := u.SessionFactory.Session(cfg.Providers)
	if err != nil {
		return failValidation(err)
	}

	cell.ResumeCreation()
	if err := replaceCell(ctx, u.State, cell); err != nil {
		return domain.Cell{}, err
	}
	runner := cellCreationRunner{
		State:      u.State,
		Files:      u.Files,
		Source:     source,
		Containers: containers,
		Session:    session,
		RetryBase:  &stored,
	}
	if err := runner.run(ctx, &cell, template, true); err != nil {
		return domain.Cell{}, err
	}
	return cell, nil
}

func resolveCell(cells []domain.Cell, identifier string) (domain.Cell, bool) {
	for _, cell := range cells {
		if cell.ID == identifier || cell.Issue == identifier || cell.Name == identifier {
			return cell, true
		}
	}
	return domain.Cell{}, false
}

func nextCreationStage(cell domain.Cell) domain.CreationStage {
	for _, stage := range []domain.CreationStage{
		domain.CreationStageSource,
		domain.CreationStageFiles,
		domain.CreationStageContainers,
		domain.CreationStageSession,
	} {
		if !cell.CreationStageCompleted(stage) {
			return stage
		}
	}
	return domain.CreationStageSession
}

func refreshRetryCell(stored domain.Cell, rendered domain.Cell) domain.Cell {
	refreshed := rendered
	refreshed.ID = stored.ID
	refreshed.Issue = stored.Issue
	refreshed.Name = stored.Name
	refreshed.Template = stored.Template
	refreshed.Branch = stored.Branch
	refreshed.Source.Path = stored.Source.Path
	refreshed.Creation = stored.Creation
	if stored.CreationStageCompleted(domain.CreationStageSource) {
		refreshed.Base = stored.Base
		refreshed.BranchMode = stored.BranchMode
		refreshed.Source = stored.Source
	}
	if stored.CreationStageCompleted(domain.CreationStageContainers) {
		refreshed.Containers = stored.Containers
	} else {
		refreshed.Containers.Network = stored.Containers.Network
		for role, current := range stored.Containers.Services {
			updated, ok := refreshed.Containers.Services[role]
			if !ok {
				continue
			}
			updated.ContainerName = current.ContainerName
			refreshed.Containers.Services[role] = updated
		}
	}
	if stored.CreationStageCompleted(domain.CreationStageSession) {
		refreshed.Session = stored.Session
	} else {
		refreshed.Session.Name = stored.Session.Name
	}
	return refreshed
}
