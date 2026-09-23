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
	if id == "" {
		return domain.Cell{}, fmt.Errorf("cell id is required")
	}
	if input.Issue == "" {
		return domain.Cell{}, fmt.Errorf("issue is required")
	}
	if input.Note != nil {
		if _, err := domain.NormalizeCellNote(*input.Note); err != nil {
			return domain.Cell{}, err
		}
	}
	name := domain.NewCellName(input.Issue)
	resolved, err := cfg.Resolve(input.Template, domain.NewTemplateVars(input.Issue, name.Value, cfg.ProjectName, input.Command))
	if err != nil {
		return domain.Cell{}, err
	}
	existing, err := u.Cells.LoadCells(ctx)
	if err != nil {
		return domain.Cell{}, err
	}
	if err := domain.EnsureCellUnique(existing, input.Issue, name); err != nil {
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
	sourceItems, sourceErr := domain.CreateSourcesService(ctx, resolved.Sources, input.Issue, source)
	sources := domain.NewSources(cfg.SourceDriverType, sourceItems)
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
	if input.Note != nil {
		if err = cell.SetNote(*input.Note); err != nil {
			return domain.Cell{}, err
		}
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

	if sourceErr != nil {
		return cell, sourceErr
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
	networks, err := containerPort.CreateContainers(ctx, resolved.Containers, cell.Name().Value, cell.Project, cell.ContainerNetworkName(), cell.WorkingDirectory())
	if err != nil {
		return domain.Cell{}, err
	}
	cell.RecordContainerNetworks(networks)
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
	sessionName, sessionCellName, project, label, _ := cell.SessionPreparation()
	if err := sessionPort.CreateSession(ctx, resolved.Session, sessionName, sessionCellName, project, label, cell.WorkingDirectory()); err != nil {
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
