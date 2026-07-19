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
			Container:     "docker",
			Session:       "tmux",
			Notifications: "tmux",
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
			"planning": {
				Name: "planning",
				Repository: domain.RepositoryTemplate{
					BranchPrefix: "",
					Base:         "main",
				},
				Containers: domain.ContainerTemplate{Services: map[string]domain.ContainerServiceTemplate{}},
				Session: domain.SessionTemplate{Windows: []domain.SessionWindowTemplate{
					{
						Name:    "planning",
						Command: `codex "Read GitHub issue #{{.issue}} first. Use superpowers:brainstorming to refine the design and use the structure of superpowers:writing-plans to make the issue body implementation-ready, but do not save separate .md design or plan files. Update the GitHub issue itself so its body is the single source of truth for design, implementation direction, and acceptance criteria."`,
					},
				}},
			},
			"implementation": {
				Name: "implementation",
				Repository: domain.RepositoryTemplate{
					BranchPrefix: "",
					Base:         "main",
				},
				Containers: domain.ContainerTemplate{Services: map[string]domain.ContainerServiceTemplate{}},
				Session: domain.SessionTemplate{Windows: []domain.SessionWindowTemplate{
					{
						Name:    "implementation",
						Command: `codex "Read GitHub issue #{{.issue}} and treat its body as the single source of truth. Use superpowers:executing-plans to implement without subagents. Before claiming completion, use superpowers:verification-before-completion. Then use superpowers:finishing-a-development-branch, create a pull request, and report the PR URL to the user."`,
					},
				}},
			},
		},
	}
	if err := u.Config.SaveConfig(ctx, cfg); err != nil {
		return domain.Config{}, err
	}
	return cfg, nil
}
