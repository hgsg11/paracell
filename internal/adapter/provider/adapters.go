package provider

import (
	"fmt"

	"github.com/shige1114/paradev/internal/adapter/container"
	"github.com/shige1114/paradev/internal/adapter/session"
	"github.com/shige1114/paradev/internal/adapter/source"
	"github.com/shige1114/paradev/internal/adapter/system"
	"github.com/shige1114/paradev/internal/domain"
	"github.com/shige1114/paradev/internal/usecase"
)

type Adapters struct {
	Source     usecase.SourcePort
	Containers usecase.ContainerPort
	Session    usecase.SessionPort
}

func NewAdapters(providers domain.ProviderConfig, runner system.Runner) (Adapters, error) {
	var adapters Adapters
	switch providers.Source {
	case "git":
		adapters.Source = source.GitSourceAdapter{Runner: runner}
	default:
		return Adapters{}, fmt.Errorf("unsupported providers.source %q", providers.Source)
	}
	switch providers.Container {
	case "":
		adapters.Containers = container.NoopAdapter{}
	case "docker":
		adapters.Containers = container.DockerCLIAdapter{Runner: runner}
	default:
		return Adapters{}, fmt.Errorf("unsupported providers.container %q", providers.Container)
	}
	switch providers.Session {
	case "tmux":
		adapters.Session = session.TmuxAdapter{Runner: runner}
	default:
		return Adapters{}, fmt.Errorf("unsupported providers.session %q", providers.Session)
	}
	return adapters, nil
}
