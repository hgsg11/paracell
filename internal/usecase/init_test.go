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
	if cfg.Providers.Container != "" {
		t.Fatalf("providers.container = %q, want empty", cfg.Providers.Container)
	}
	if cfg.Providers.Session != "tmux" {
		t.Fatalf("providers.session = %q, want %q", cfg.Providers.Session, "tmux")
	}
	if cfg.Providers.Notifications != "tmux" {
		t.Fatalf("providers.notifications = %q, want %q", cfg.Providers.Notifications, "tmux")
	}
	if len(cfg.Templates) != 4 {
		t.Fatalf("templates length = %d, want 4", len(cfg.Templates))
	}
	for name, branchPrefix := range map[string]string{
		"feat":   "feat/",
		"update": "update/",
		"fix":    "fix/",
		"review": "review/",
	} {
		template, ok := cfg.Templates[name]
		if !ok {
			t.Fatalf("template %q is missing", name)
		}
		if template.Name != name {
			t.Fatalf("template name = %q, want %q", template.Name, name)
		}
		if template.Repository.Base != "main" {
			t.Fatalf("%s repository.base = %q, want main", name, template.Repository.Base)
		}
		if template.Repository.BranchPrefix != branchPrefix {
			t.Fatalf("%s repository.branchPrefix = %q, want %q", name, template.Repository.BranchPrefix, branchPrefix)
		}
		if len(template.Containers.Services) != 0 {
			t.Fatalf("%s containers.services length = %d, want 0", name, len(template.Containers.Services))
		}
		if len(template.Session.Windows) != 0 {
			t.Fatalf("%s session windows length = %d, want 0", name, len(template.Session.Windows))
		}
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
