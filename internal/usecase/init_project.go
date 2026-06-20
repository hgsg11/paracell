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
						Command: `codex "Read GitHub issue #{{.issue}} first. Use superpowers:brainstorming to turn it into an approved design spec, then use superpowers:writing-plans to create the implementation plan. Work from the current repository context and ask the user for any approvals required by those skills."`,
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
						Command: `codex "Read GitHub issue #{{.issue}} and the existing implementation plan first. Use superpowers:executing-plans to implement without subagents. Before claiming completion, use superpowers:verification-before-completion. Then use superpowers:finishing-a-development-branch, create a pull request, and report the PR URL to the user."`,
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
