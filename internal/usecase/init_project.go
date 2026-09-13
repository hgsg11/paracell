package usecase

import (
	"context"

	"github.com/hgsg11/paracell/internal/domain"
)

type InitProjectUseCase struct {
	Config InitConfigPort
	State  StateInitializer
}

func (u InitProjectUseCase) Execute(ctx context.Context) (domain.Templates, error) {
	exists, err := u.Config.ConfigExists(ctx)
	if err != nil {
		return domain.Templates{}, err
	}
	if err := u.State.Initialize(ctx); err != nil {
		return domain.Templates{}, err
	}
	if exists {
		return domain.Templates{}, nil
	}
	sessionDriver, err := domain.NewSessionDriverType("tmux")
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
		template, err := domain.NewTemplate(name, []domain.SourceTemplate{source}, nil, domain.NewSessionTemplate(nil))
		if err != nil {
			return domain.Templates{}, err
		}
		items = append(items, template)
	}
	cfg, err := domain.NewTemplates("", items, sessionDriver, domain.NewContainerDriverType(""), sourceDriver, notificationDriver)
	if err != nil {
		return domain.Templates{}, err
	}
	if err := u.Config.SaveConfig(ctx, cfg); err != nil {
		return domain.Templates{}, err
	}
	return cfg, nil
}
