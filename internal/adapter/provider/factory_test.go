package provider

import (
	"testing"

	"github.com/shige1114/paradev/internal/domain"
)

func TestFactorySourceは対応Providerを選択できる(t *testing.T) {
	adapter, err := Factory{}.Source(domain.ProviderConfig{Source: "git"})
	if err != nil {
		t.Fatalf("provider source選択でエラーが返った: %v", err)
	}
	if adapter == nil {
		t.Fatal("source adapter is nil")
	}
}

func TestFactorySourceは未対応Providerをエラーにする(t *testing.T) {
	_, err := Factory{}.Source(domain.ProviderConfig{Source: "svn"})
	if err == nil {
		t.Fatal("未対応providerなのにエラーが返らなかった")
	}
	if err.Error() != `unsupported providers.source "svn"` {
		t.Fatalf("error = %q, want %q", err.Error(), `unsupported providers.source "svn"`)
	}
}

func TestFactoryContainerは対応Providerを選択できる(t *testing.T) {
	adapter, err := Factory{}.Container(domain.ProviderConfig{Container: "docker"})
	if err != nil {
		t.Fatalf("provider container選択でエラーが返った: %v", err)
	}
	if adapter == nil {
		t.Fatal("container adapter is nil")
	}
}

func TestFactoryContainerは空ならNoopを返す(t *testing.T) {
	adapter, err := Factory{}.Container(domain.ProviderConfig{})
	if err != nil {
		t.Fatalf("provider container選択でエラーが返った: %v", err)
	}
	if adapter == nil {
		t.Fatal("container adapter is nil")
	}
}

func TestFactoryContainerは未対応Providerをエラーにする(t *testing.T) {
	_, err := Factory{}.Container(domain.ProviderConfig{Container: "podman"})
	if err == nil {
		t.Fatal("未対応container providerなのにエラーが返らなかった")
	}
	if err.Error() != `unsupported providers.container "podman"` {
		t.Fatalf("error = %q, want %q", err.Error(), `unsupported providers.container "podman"`)
	}
}

func TestFactorySessionは対応Providerを選択できる(t *testing.T) {
	adapter, err := Factory{}.Session(domain.ProviderConfig{Session: "tmux"})
	if err != nil {
		t.Fatalf("provider session選択でエラーが返った: %v", err)
	}
	if adapter == nil {
		t.Fatal("session adapter is nil")
	}
}

func TestFactorySessionは未対応Providerをエラーにする(t *testing.T) {
	_, err := Factory{}.Session(domain.ProviderConfig{Session: "screen"})
	if err == nil {
		t.Fatal("未対応session providerなのにエラーが返らなかった")
	}
	if err.Error() != `unsupported providers.session "screen"` {
		t.Fatalf("error = %q, want %q", err.Error(), `unsupported providers.session "screen"`)
	}
}
