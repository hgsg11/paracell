package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/hgsg11/paracell/internal/domain"
)

func TestInitは現在のProject情報から設定を作成して保存する(t *testing.T) {
	ctx := context.Background()
	ports := &fakeInitPorts{}
	uc := InitProjectUseCase{
		Config: ports,
		State:  ports,
	}

	cfg, err := uc.Execute(ctx)

	if err != nil {
		t.Fatalf("initでエラーが返った: %v", err)
	}
	if !ports.saved {
		t.Fatal("設定が保存されなかった")
	}
	if !ports.initialized {
		t.Fatal("state databaseが初期化されなかった")
	}
	if cfg.ProjectName != "" {
		t.Fatalf("project.name = %q, want empty", cfg.ProjectName)
	}
	if cfg.SourceDriverType != domain.Git {
		t.Fatalf("providers.source = %q, want %q", cfg.SourceDriverType, domain.Git)
	}
	if cfg.ContainerDriverType != domain.None {
		t.Fatalf("providers.container = %q, want none", cfg.ContainerDriverType)
	}
	if cfg.SessionDriverType != domain.Tmux {
		t.Fatalf("providers.session = %q, want %q", cfg.SessionDriverType, domain.Tmux)
	}
	if cfg.NotificationDriverType != domain.TmuxNotification {
		t.Fatalf("providers.notifications = %q, want %q", cfg.NotificationDriverType, domain.TmuxNotification)
	}
	if len(cfg.Templates) != 4 {
		t.Fatalf("templates length = %d, want 4", len(cfg.Templates))
	}
	for _, template := range cfg.Templates {
		if template.Repository == nil || template.Repository.Base != "main" || template.Repository.Prefix != template.Name+"/" {
			t.Fatalf("template = %#v", template)
		}
	}
}

func TestInitは既存設定を上書きせずStateDatabaseを初期化する(t *testing.T) {
	ctx := context.Background()
	ports := &fakeInitPorts{exists: true}
	uc := InitProjectUseCase{
		Config: ports,
		State:  ports,
	}

	_, err := uc.Execute(ctx)

	if err != nil {
		t.Fatalf("initでエラーが返った: %v", err)
	}
	if ports.saved {
		t.Fatal("既存設定があるのに保存された")
	}
	if !ports.initialized {
		t.Fatal("state databaseが初期化されなかった")
	}
}

func TestInitはStateDatabaseを初期化できない場合に設定を保存しない(t *testing.T) {
	ctx := context.Background()
	ports := &fakeInitPorts{initializeErr: errors.New("migration failed")}
	uc := InitProjectUseCase{Config: ports, State: ports}

	_, err := uc.Execute(ctx)

	if err == nil {
		t.Fatal("state databaseを初期化できないのにエラーが返らなかった")
	}
	if ports.saved {
		t.Fatal("state databaseを初期化できないのに設定が保存された")
	}
}

type fakeInitPorts struct {
	exists        bool
	saved         bool
	initialized   bool
	initializeErr error
}

func (f *fakeInitPorts) ConfigExists(ctx context.Context) (bool, error) {
	return f.exists, nil
}

func (f *fakeInitPorts) SaveConfig(ctx context.Context, cfg domain.Templates) error {
	f.saved = true
	return nil
}

func (f *fakeInitPorts) Initialize(ctx context.Context) error {
	f.initialized = true
	return f.initializeErr
}
