package provider

import (
	"testing"

	"github.com/hgsg11/paracell/internal/adapter/container"
	"github.com/hgsg11/paracell/internal/adapter/notification"
	"github.com/hgsg11/paracell/internal/adapter/session"
	"github.com/hgsg11/paracell/internal/adapter/source"
	"github.com/hgsg11/paracell/internal/domain"
)

func TestFactoryはDriverTypeからAdapterを作る(t *testing.T) {
	factory := Factory{}
	if got, err := factory.Source(domain.Git); err != nil {
		t.Fatal(err)
	} else if _, ok := got.(source.GitSourceAdapter); !ok {
		t.Fatalf("source = %T", got)
	}
	if got, err := factory.Container(domain.Docker); err != nil {
		t.Fatal(err)
	} else if _, ok := got.(container.DockerCLIAdapter); !ok {
		t.Fatalf("container = %T", got)
	}
	if got, err := factory.Session(domain.Tmux); err != nil {
		t.Fatal(err)
	} else if _, ok := got.(session.TmuxAdapter); !ok {
		t.Fatalf("session = %T", got)
	}
	if got, err := factory.Notification(domain.NoNotification); err != nil {
		t.Fatal(err)
	} else if _, ok := got.(notification.NoopNotifier); !ok {
		t.Fatalf("notification = %T", got)
	}
}
