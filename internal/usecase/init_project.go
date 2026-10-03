package usecase

import (
	"context"

	"github.com/hgsg11/paracell/internal/domain"
)

type InitProjectUseCase struct {
	Config InitConfigPort
	Cells  CellInitializer
}

func (u InitProjectUseCase) Execute(ctx context.Context) (domain.Templates, error) {
	exists, err := u.Config.ConfigExists(ctx)
	if err != nil {
		return domain.Templates{}, err
	}
	if err := u.Cells.Initialize(ctx); err != nil {
		return domain.Templates{}, err
	}
	if exists {
		return domain.Templates{}, nil
	}
	workspaceDriver, err := domain.NewWorkspaceDriverType("tmux")
	if err != nil {
		return domain.Templates{}, err
	}
	sourceDriver, err := domain.NewSourceDriverType("git")
	if err != nil {
		return domain.Templates{}, err
	}
	notificationDriver, err := domain.NewNotificationDriverType("tmux")
	if err != nil {
		return domain.Templates{}, err
	}
	names := []string{"feat", "update", "fix", "review"}
	items := make([]domain.Template, 0, len(names))
	for _, name := range names {
		source, err := domain.NewSourceTemplate(".", "main", name+"/")
		if err != nil {
			return domain.Templates{}, err
		}
		target, err := domain.NewTargetCellSpec("repository", &source, nil, nil)
		if err != nil {
			return domain.Templates{}, err
		}
		commander, err := domain.NewCommanderCellSpec("workspace", domain.NewWorkspaceTemplate(nil), []domain.TargetCellSpec{target}, nil)
		if err != nil {
			return domain.Templates{}, err
		}
		template, err := domain.NewUnresolvedTemplate(name, "", false, &commander)
		if err != nil {
			return domain.Templates{}, err
		}
		items = append(items, template)
	}
	cfg, err := domain.NewTemplates("", items, workspaceDriver, domain.NewContainerDriverType(""), sourceDriver, notificationDriver)
	if err != nil {
		return domain.Templates{}, err
	}
	if err := u.Config.SaveConfig(ctx, cfg); err != nil {
		return domain.Templates{}, err
	}
	return cfg, nil
}
