package provider

import (
	"fmt"

	"github.com/hgsg11/paracell/internal/adapter/container"
	"github.com/hgsg11/paracell/internal/adapter/notification"
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

func NewFactory(runner system.Runner, root string) Factory {
	return Factory{Runner: runner, Root: root}
}

func (f Factory) Source(driver domain.SourceDriverType) (usecase.SourcePort, error) {
	switch driver {
	case domain.Git:
		return source.NewGitSourceAdapter(f.Runner, f.Root), nil
	default:
		return nil, fmt.Errorf("unsupported source driver %q", driver)
	}
}

func (f Factory) Container(driver domain.ContainerDriverType) (usecase.ContainerPort, error) {
	switch driver {
	case domain.None:
		return container.NewNoopAdapter(), nil
	case domain.Docker:
		return container.NewDockerCLIAdapter(f.Runner, f.Root), nil
	default:
		return nil, fmt.Errorf("unsupported container driver %q", driver)
	}
}

func (f Factory) Session(driver domain.SessionDriverType) (usecase.SessionPort, error) {
	switch driver {
	case domain.Tmux:
		return session.NewTmuxAdapter(f.Runner, f.Root), nil
	default:
		return nil, fmt.Errorf("unsupported session driver %q", driver)
	}
}

func (f Factory) Notification(driver domain.NotificationDriverType) (usecase.Notifier, error) {
	switch driver {
	case domain.NoNotification:
		return notification.NewNoopNotifier(), nil
	case domain.TmuxNotification:
		return notification.NewTmuxNotifier(f.Runner), nil
	default:
		return nil, fmt.Errorf("unsupported notification driver %q", driver)
	}
}
