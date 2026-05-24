package provider

import (
	"fmt"

	"github.com/hgsg11/paracell/internal/adapter/container"
	"github.com/hgsg11/paracell/internal/adapter/session"
	"github.com/hgsg11/paracell/internal/adapter/source"
	"github.com/hgsg11/paracell/internal/adapter/system"
	"github.com/hgsg11/paracell/internal/domain"
	"github.com/hgsg11/paracell/internal/usecase"
)

type Factory struct {
	Runner system.Runner
	Root   string
}

func (f Factory) Source(provider domain.ProviderConfig) (usecase.SourcePort, error) {
	switch provider.Source {
	case "git":
		return source.GitSourceAdapter{Runner: f.Runner}, nil
	default:
		return nil, fmt.Errorf("unsupported providers.source %q", provider.Source)
	}
}

func (f Factory) Container(provider domain.ProviderConfig) (usecase.ContainerPort, error) {
	switch provider.Container {
	case "":
		return container.NoopAdapter{}, nil
	case "docker":
		return container.DockerCLIAdapter{Runner: f.Runner, Root: f.Root}, nil
	default:
		return nil, fmt.Errorf("unsupported providers.container %q", provider.Container)
	}
}

func (f Factory) Session(provider domain.ProviderConfig) (usecase.SessionPort, error) {
	switch provider.Session {
	case "tmux":
		return session.TmuxAdapter{Runner: f.Runner}, nil
	default:
		return nil, fmt.Errorf("unsupported providers.session %q", provider.Session)
	}
}
