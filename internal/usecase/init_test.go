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
