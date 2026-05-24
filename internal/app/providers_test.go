package app

import (
	"testing"

	"github.com/shige1114/paradev/internal/domain"
)

func TestProviderAdaptersは対応Providerを選択できる(t *testing.T) {
	adapters, err := NewProviderAdapters(domain.ProviderConfig{
		Source:    "git",
		Container: "docker",
		Session:   "tmux",
	}, nil)

	if err != nil {
		t.Fatalf("provider adapter選択でエラーが返った: %v", err)
	}
	if adapters.Source == nil {
		t.Fatal("source adapter is nil")
	}
	if adapters.Containers == nil {
		t.Fatal("container adapter is nil")
	}
	if adapters.Session == nil {
		t.Fatal("session adapter is nil")
	}
}

func TestProviderAdaptersは未対応Providerをエラーにする(t *testing.T) {
	_, err := NewProviderAdapters(domain.ProviderConfig{
		Source:    "svn",
		Container: "docker",
		Session:   "tmux",
	}, nil)

	if err == nil {
		t.Fatal("未対応providerなのにエラーが返らなかった")
	}
	if err.Error() != `unsupported providers.source "svn"` {
		t.Fatalf("error = %q, want %q", err.Error(), `unsupported providers.source "svn"`)
	}
}
