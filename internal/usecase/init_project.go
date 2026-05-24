package usecase

import (
	"context"
	"errors"

	"github.com/shige1114/paradev/internal/domain"
)

type InitConfig = domain.Config

type InitProjectUseCase struct {
	Config InitConfigPort
}

func (u InitProjectUseCase) Execute(ctx context.Context) (domain.Config, error) {
	exists, err := u.Config.ConfigExists(ctx)
	if err != nil {
		return domain.Config{}, err
	}
	if exists {
		return domain.Config{}, errors.New(".pdev.yml already exists")
	}
	cfg := domain.Config{
		Project: domain.ProjectConfig{Name: ""},
		Providers: domain.ProviderConfig{
			Source:    "git",
			Container: "docker",
			Session:   "tmux",
		},
		Templates: map[string]domain.Template{
			"default": {
				Name: "default",
				Repository: domain.RepositoryTemplate{
					BranchPrefix: "feat/",
					Base:         "main",
				},
				Containers: domain.ContainerTemplate{Services: map[string]domain.ContainerServiceTemplate{}},
				Session:    domain.SessionTemplate{Windows: []domain.SessionWindowTemplate{}},
			},
		},
	}
	if err := u.Config.SaveConfig(ctx, cfg); err != nil {
		return domain.Config{}, err
	}
	return cfg, nil
}
