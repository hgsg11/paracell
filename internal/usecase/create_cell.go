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
	Config     ConfigPort
	State      CellStatePort
	Source     SourcePort
	Containers ContainerPort
	Session    SessionPort
	IDs        IDGenerator
}

func (u CreateCellUseCase) Execute(ctx context.Context, input CreateCellInput) (domain.Cell, error) {
	cfg, err := u.Config.Load(ctx)
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
	if err := u.Source.CreateSource(ctx, cell); err != nil {
		return domain.Cell{}, err
	}
	if err := u.Containers.CreateContainers(ctx, cell, template); err != nil {
		return domain.Cell{}, err
	}
	if err := u.Session.CreateSession(ctx, cell); err != nil {
		return domain.Cell{}, err
	}
	existing = append(existing, cell)
	if err := u.State.SaveCells(ctx, existing); err != nil {
		return domain.Cell{}, err
	}
	return cell, nil
}
