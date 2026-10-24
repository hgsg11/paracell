package usecase

import (
	"context"

	"github.com/hgsg11/paracell/internal/domain"
)

type InitProjectUseCase struct {
	Config     InitConfigPort
	Cells      CellInitializer
	AgentHooks AgentHookPort
}

// Execute saves paracell.yaml only when it does not exist yet, and installs the
// agent status hooks in both cases. The hooks come after the config so that a
// failed hook install leaves the config in place and a rerun only adds the hooks.
func (u InitProjectUseCase) Execute(ctx context.Context) (domain.Templates, error) {
	exists, err := u.Config.ConfigExists(ctx)
	if err != nil {
		return domain.Templates{}, err
	}
	if err := u.Cells.Initialize(ctx); err != nil {
		return domain.Templates{}, err
	}
	cfg := domain.Templates{}
	if !exists {
		cfg, err = u.saveDefaultConfig(ctx)
		if err != nil {
			return domain.Templates{}, err
		}
	}
	if err := u.AgentHooks.InstallStatusHooks(ctx, domain.AgentStatusHooks()); err != nil {
		return domain.Templates{}, err
	}
	return cfg, nil
}

func (u InitProjectUseCase) saveDefaultConfig(ctx context.Context) (domain.Templates, error) {
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
	names := []string{"self"}
	items := make([]domain.Template, 0, len(names))
	for _, name := range names {
		source, err := domain.NewSourceTemplate(".", "main")
		if err != nil {
			return domain.Templates{}, err
		}
		target, err := domain.NewTargetCellSpec("repository", &source, nil)
		if err != nil {
			return domain.Templates{}, err
		}
		agent, err := domain.NewWindow("agent", `claude "{{.Command}}"`)
		if err != nil {
			return domain.Templates{}, err
		}
		commander, err := domain.NewCommanderCellSpec("workspace", domain.NewWorkspaceTemplate([]domain.Window{agent}))
		if err != nil {
			return domain.Templates{}, err
		}
		template, err := domain.NewUnresolvedTemplate(name, &commander, []domain.TargetCellSpec{target}, nil)
		if err != nil {
			return domain.Templates{}, err
		}
		items = append(items, template)
	}
	cfg, err := domain.NewTemplates("", items, workspaceDriver, domain.NewContainerDriverType(""), sourceDriver, notificationDriver, nil)
	if err != nil {
		return domain.Templates{}, err
	}
	if err := u.Config.SaveConfig(ctx, cfg); err != nil {
		return domain.Templates{}, err
	}
	return cfg, nil
}
