package usecase

import (
	"context"
	"fmt"

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

func (u ForkCellUseCase) Execute(ctx context.Context, input ForkCellInput) (domain.Cell, error) {
	cfg, err := u.Config.Load(ctx)
	if err != nil {
		return domain.Cell{}, err
	}
	id := u.IDs.NewID()
	name := domain.NewCellName(input.Issue)
	resolved, err := cfg.Resolve(input.Template, domain.NewTemplateVars(input.Issue, name.Value, cfg.ProjectName, input.Command))
	if err != nil {
		return domain.Cell{}, err
	}
	existing, err := u.Cells.LoadCells(ctx)
	if err != nil {
		return domain.Cell{}, err
	}
	sources, err := domain.BuildSources(cfg.SourceDriverType, resolved.Sources, input.Issue)
	if err != nil {
		return domain.Cell{}, err
	}
	containers, err := domain.BuildContainers(cfg.ContainerDriverType, resolved.Containers)
	if err != nil {
		return domain.Cell{}, err
	}
	session, err := domain.BuildSession(cfg.SessionDriverType, resolved.Session)
	if err != nil {
		return domain.Cell{}, err
	}
	cell, err := domain.NewCell(
		id, input.Issue, cfg.ProjectName, input.Template,
		sources,
		containers,
		session,
		cfg.NotificationDriverType,
	)
	if err != nil {
		return domain.Cell{}, err
	}
	source, err := u.SourceFactory.Source(cfg.SourceDriverType)
	if err != nil {
		return domain.Cell{}, err
	}
	containerPort, err := u.ContainerFactory.Container(cfg.ContainerDriverType)
	if err != nil {
		return domain.Cell{}, err
	}
	sessionPort, err := u.SessionFactory.Session(cfg.SessionDriverType)
	if err != nil {
		return domain.Cell{}, err
	}
	if input.Note != nil {
		if err = cell.SetNote(*input.Note); err != nil {
			return domain.Cell{}, err
		}
	}
	if err := domain.EnsureCellUnique(existing, input.Issue, cell.Name()); err != nil {
		return domain.Cell{}, err
	}
	cell.BeginCreation()
	if err := u.Cells.UpdateCells(ctx, func(latest []domain.Cell) ([]domain.Cell, error) {
		if err := domain.EnsureCellUnique(latest, input.Issue, cell.Name()); err != nil {
			return nil, err
		}
		return append(latest, cell), nil
	}); err != nil {
		return domain.Cell{}, err
	}

	if err := domain.CreateSourcesService(ctx, resolved.Sources, input.Issue, source); err != nil {
		return domain.Cell{}, err
	}
	if err := u.Cells.UpdateCells(ctx, func(cells []domain.Cell) ([]domain.Cell, error) {
		for i := range cells {
			if cells[i].SameIdentity(cell) {
				cells[i] = cell
				return cells, nil
			}
		}
		return nil, fmt.Errorf("cell %q not found", cell.Name().Value)
	}); err != nil {
		return domain.Cell{}, fmt.Errorf("save source stage: %w", err)
	}
	if err := cell.AdvanceVersion(); err != nil {
		return domain.Cell{}, err
	}
	if err := domain.CreateContainersService(ctx, &cell, resolved.Containers, containerPort.CreateContainers); err != nil {
		return domain.Cell{}, err
	}
	if err := u.Cells.UpdateCells(ctx, func(cells []domain.Cell) ([]domain.Cell, error) {
		for i := range cells {
			if cells[i].SameIdentity(cell) {
				cells[i] = cell
				return cells, nil
			}
		}
		return nil, fmt.Errorf("cell %q not found", cell.Name().Value)
	}); err != nil {
		return domain.Cell{}, fmt.Errorf("save containers stage: %w", err)
	}
	if err := cell.AdvanceVersion(); err != nil {
		return domain.Cell{}, err
	}
	if err := domain.CreateSessionService(ctx, cell, resolved.Session, sessionPort.CreateSession); err != nil {
		return domain.Cell{}, err
	}
	cell.FinishCreation()
	if err := u.Cells.UpdateCells(ctx, func(cells []domain.Cell) ([]domain.Cell, error) {
		for i := range cells {
			if cells[i].SameIdentity(cell) {
				cells[i] = cell
				return cells, nil
			}
		}
		return nil, fmt.Errorf("cell %q not found", cell.Name().Value)
	}); err != nil {
		return domain.Cell{}, fmt.Errorf("save session stage: %w", err)
	}
	if err := cell.AdvanceVersion(); err != nil {
		return domain.Cell{}, err
	}
	return cell, nil
}
