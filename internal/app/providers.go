package app

import (
	"context"
	"fmt"

	"github.com/shige1114/paradev/internal/adapter/container"
	"github.com/shige1114/paradev/internal/adapter/session"
	"github.com/shige1114/paradev/internal/adapter/source"
	"github.com/shige1114/paradev/internal/adapter/system"
	"github.com/shige1114/paradev/internal/domain"
	"github.com/shige1114/paradev/internal/usecase"
)

type ProviderAdapters struct {
	Source     usecase.SourcePort
	Containers usecase.ContainerPort
	Session    usecase.SessionPort
}

func NewProviderAdapters(providers domain.ProviderConfig, runner system.Runner) (ProviderAdapters, error) {
	var adapters ProviderAdapters
	switch providers.Source {
	case "git":
		adapters.Source = source.GitSourceAdapter{Runner: runner}
	default:
		return ProviderAdapters{}, fmt.Errorf("unsupported providers.source %q", providers.Source)
	}
	switch providers.Container {
	case "docker":
		adapters.Containers = container.DockerCLIAdapter{Runner: runner}
	default:
		return ProviderAdapters{}, fmt.Errorf("unsupported providers.container %q", providers.Container)
	}
	switch providers.Session {
	case "tmux":
		adapters.Session = session.TmuxAdapter{Runner: runner}
	default:
		return ProviderAdapters{}, fmt.Errorf("unsupported providers.session %q", providers.Session)
	}
	return adapters, nil
}

type staticConfig struct {
	cfg domain.Config
}

func (s staticConfig) Load(ctx context.Context) (domain.Config, error) {
	return s.cfg, nil
}
