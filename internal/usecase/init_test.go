package usecase

import (
	"context"
	"testing"
)

func TestInitは現在のProject情報から設定を作成して保存する(t *testing.T) {
	ctx := context.Background()
	ports := &fakeInitPorts{}
	uc := InitProjectUseCase{
		Config: ports,
	}

	cfg, err := uc.Execute(ctx)

	if err != nil {
		t.Fatalf("initでエラーが返った: %v", err)
	}
	if !ports.saved {
		t.Fatal("設定が保存されなかった")
	}
	if cfg.Project.Name != "" {
		t.Fatalf("project.name = %q, want empty", cfg.Project.Name)
	}
	if cfg.Providers.Source != "git" {
		t.Fatalf("providers.source = %q, want %q", cfg.Providers.Source, "git")
	}
	if cfg.Providers.Container != "docker" {
		t.Fatalf("providers.container = %q, want %q", cfg.Providers.Container, "docker")
	}
	if cfg.Providers.Session != "tmux" {
		t.Fatalf("providers.session = %q, want %q", cfg.Providers.Session, "tmux")
	}
	if cfg.Providers.Notifications != "tmux" {
		t.Fatalf("providers.notifications = %q, want %q", cfg.Providers.Notifications, "tmux")
	}
	template := cfg.Templates["default"]
	if template.Repository.Base != "main" {
		t.Fatalf("repository.base = %q, want %q", template.Repository.Base, "main")
	}
	if template.Repository.BranchPrefix != "feat/" {
		t.Fatalf("repository.branchPrefix = %q, want %q", template.Repository.BranchPrefix, "feat/")
	}
	if len(template.Containers.Services) != 0 {
		t.Fatalf("containers.services length = %d, want 0", len(template.Containers.Services))
	}
	if len(template.Session.Windows) != 0 {
		t.Fatalf("session windows length = %d, want 0", len(template.Session.Windows))
	}
	planning := cfg.Templates["planning"]
	if planning.Repository.Base != "main" {
		t.Fatalf("planning repository.base = %q, want %q", planning.Repository.Base, "main")
	}
	if planning.Repository.BranchPrefix != "" {
		t.Fatalf("planning repository.branchPrefix = %q, want empty", planning.Repository.BranchPrefix)
	}
	if len(planning.Session.Windows) != 1 {
		t.Fatalf("planning session windows length = %d, want 1", len(planning.Session.Windows))
	}
	if planning.Session.Windows[0].Command == "" {
		t.Fatal("planning session command is empty")
	}
	implementation := cfg.Templates["implementation"]
	if implementation.Repository.Base != "main" {
		t.Fatalf("implementation repository.base = %q, want %q", implementation.Repository.Base, "main")
	}
	if implementation.Repository.BranchPrefix != "" {
		t.Fatalf("implementation repository.branchPrefix = %q, want empty", implementation.Repository.BranchPrefix)
	}
	if len(implementation.Session.Windows) != 1 {
		t.Fatalf("implementation session windows length = %d, want 1", len(implementation.Session.Windows))
	}
	if implementation.Session.Windows[0].Command == "" {
		t.Fatal("implementation session command is empty")
	}
}

func TestInitは既存設定がある場合に失敗する(t *testing.T) {
	ctx := context.Background()
	ports := &fakeInitPorts{exists: true}
	uc := InitProjectUseCase{
		Config: ports,
	}

	_, err := uc.Execute(ctx)

	if err == nil {
		t.Fatal("既存設定があるのにエラーが返らなかった")
	}
	if ports.saved {
		t.Fatal("既存設定があるのに保存された")
	}
}

type fakeInitPorts struct {
	exists bool
	saved  bool
}

func (f *fakeInitPorts) ConfigExists(ctx context.Context) (bool, error) {
	return f.exists, nil
}

func (f *fakeInitPorts) SaveConfig(ctx context.Context, cfg InitConfig) error {
	f.saved = true
	return nil
}
