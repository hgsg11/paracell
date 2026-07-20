package usecase

import (
	"context"
	"errors"

	"github.com/hgsg11/paracell/internal/domain"
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
		return domain.Config{}, errors.New("paracell.yaml already exists")
	}
	cfg := domain.Config{
		Project: domain.ProjectConfig{Name: ""},
		Providers: domain.ProviderConfig{
			Source:        "git",
			Session:       "tmux",
			Notifications: "tmux",
		},
		Templates: map[string]domain.Template{
			"feat": {
				Name: "feat",
				Repository: domain.RepositoryTemplate{
					BranchPrefix: "feat/",
					Base:         "main",
				},
				Session: domain.SessionTemplate{Windows: []domain.SessionWindowTemplate{}},
			},
			"update": {
				Name: "update",
				Repository: domain.RepositoryTemplate{
					BranchPrefix: "update/",
					Base:         "main",
				},
				Session: domain.SessionTemplate{Windows: []domain.SessionWindowTemplate{}},
			},
			"fix": {
				Name: "fix",
				Repository: domain.RepositoryTemplate{
					BranchPrefix: "fix/",
					Base:         "main",
				},
				Session: domain.SessionTemplate{Windows: []domain.SessionWindowTemplate{}},
			},
			"review": {
				Name: "review",
				Repository: domain.RepositoryTemplate{
					BranchPrefix: "review/",
					Base:         "main",
				},
				Session: domain.SessionTemplate{Windows: []domain.SessionWindowTemplate{}},
			},
		},
	}
	if err := u.Config.SaveConfig(ctx, cfg); err != nil {
		return domain.Config{}, err
	}
	return cfg, nil
}
