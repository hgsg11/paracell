package usecase

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/hgsg11/paracell/internal/domain"
)

func TestInitは現在のProject情報から設定を作成して保存する(t *testing.T) {
	ctx := context.Background()
	ports := &fakeInitPorts{}
	uc := InitProjectUseCase{
		Config:     ports,
		Cells:      ports,
		AgentHooks: ports,
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
	if cfg.WorkspaceDriverType != domain.Tmux {
		t.Fatalf("providers.workspace = %q, want %q", cfg.WorkspaceDriverType, domain.Tmux)
	}
	if cfg.NotificationDriverType != domain.TmuxNotification {
		t.Fatalf("providers.notifications = %q, want %q", cfg.NotificationDriverType, domain.TmuxNotification)
	}
	if len(cfg.Templates) != 1 {
		t.Fatalf("templates length = %d, want 1", len(cfg.Templates))
	}
	template := cfg.Templates[0]
	if template.Name != "self" {
		t.Fatalf("template name = %q, want self", template.Name)
	}
	if template.Commander == nil || len(template.Targets) != 1 || template.Targets[0].Name != "repository" || template.Targets[0].Source == nil || template.Targets[0].Source.Base != "main" {
		t.Fatalf("template = %#v", template)
	}
	if windows := template.Commander.Workspace.Windows; len(windows) != 1 || windows[0].Name != "agent" || windows[0].Command != `claude "{{.Command}}"` {
		t.Fatalf("commander windows = %#v", windows)
	}
	if cfg.Prefixes[domain.FeatPrefix] != "feat/" || cfg.Prefixes[domain.UpdatePrefix] != "update/" || cfg.Prefixes[domain.FixPrefix] != "fix/" {
		t.Fatalf("prefixes = %#v", cfg.Prefixes)
	}
}

func TestInitは既存設定を上書きせずStateDatabaseを初期化する(t *testing.T) {
	ctx := context.Background()
	ports := &fakeInitPorts{exists: true}
	uc := InitProjectUseCase{
		Config:     ports,
		Cells:      ports,
		AgentHooks: ports,
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
	uc := InitProjectUseCase{Config: ports, Cells: ports, AgentHooks: ports}

	_, err := uc.Execute(ctx)

	if err == nil {
		t.Fatal("state databaseを初期化できないのにエラーが返らなかった")
	}
	if ports.saved {
		t.Fatal("state databaseを初期化できないのに設定が保存された")
	}
}

func TestInitは新しいProjectで設定の保存の後にAgentのHookを入れる(t *testing.T) {
	ports := &fakeInitPorts{}
	uc := InitProjectUseCase{Config: ports, Cells: ports, AgentHooks: ports}

	if _, err := uc.Execute(context.Background()); err != nil {
		t.Fatalf("initでエラーが返った: %v", err)
	}

	wantCalls := []string{"ConfigExists", "Initialize", "SaveConfig", "InstallStatusHooks"}
	if !reflect.DeepEqual(ports.calls, wantCalls) {
		t.Fatalf("calls = %v, want %v", ports.calls, wantCalls)
	}
	if !reflect.DeepEqual(ports.installed, [][]domain.AgentStatusHook{domain.AgentStatusHooks()}) {
		t.Fatalf("installed hooks = %#v, want one call with %#v", ports.installed, domain.AgentStatusHooks())
	}
}

func TestInitは既存設定があっても設定を保存せずにAgentのHookを入れる(t *testing.T) {
	ports := &fakeInitPorts{exists: true}
	uc := InitProjectUseCase{Config: ports, Cells: ports, AgentHooks: ports}

	if _, err := uc.Execute(context.Background()); err != nil {
		t.Fatalf("initでエラーが返った: %v", err)
	}

	wantCalls := []string{"ConfigExists", "Initialize", "InstallStatusHooks"}
	if !reflect.DeepEqual(ports.calls, wantCalls) {
		t.Fatalf("calls = %v, want %v", ports.calls, wantCalls)
	}
	if len(ports.installed) != 1 || len(ports.installed[0]) != 2 {
		t.Fatalf("installed hooks = %#v, want one call with 2 hooks", ports.installed)
	}
}

func TestInitはHookを入れられないとエラーを返し保存済みの設定は残す(t *testing.T) {
	installErr := errors.New("settings.json is broken")
	ports := &fakeInitPorts{installErr: installErr}
	uc := InitProjectUseCase{Config: ports, Cells: ports, AgentHooks: ports}

	_, err := uc.Execute(context.Background())

	if !errors.Is(err, installErr) {
		t.Fatalf("error = %v, want %v", err, installErr)
	}
	if !ports.saved {
		t.Fatal("hookの導入より前に設定が保存されていない")
	}
}

func TestInitはStateDatabaseを初期化できない場合にHookを入れない(t *testing.T) {
	ports := &fakeInitPorts{initializeErr: errors.New("migration failed")}
	uc := InitProjectUseCase{Config: ports, Cells: ports, AgentHooks: ports}

	if _, err := uc.Execute(context.Background()); err == nil {
		t.Fatal("state databaseを初期化できないのにエラーが返らなかった")
	}

	if len(ports.installed) != 0 {
		t.Fatalf("installed hooks = %#v, want no call", ports.installed)
	}
}

type fakeInitPorts struct {
	exists        bool
	saved         bool
	initialized   bool
	initializeErr error
	installErr    error
	installed     [][]domain.AgentStatusHook
	calls         []string
}

func (f *fakeInitPorts) ConfigExists(ctx context.Context) (bool, error) {
	f.calls = append(f.calls, "ConfigExists")
	return f.exists, nil
}

func (f *fakeInitPorts) SaveConfig(ctx context.Context, cfg domain.Templates) error {
	f.calls = append(f.calls, "SaveConfig")
	f.saved = true
	return nil
}

func (f *fakeInitPorts) Initialize(ctx context.Context) error {
	f.calls = append(f.calls, "Initialize")
	f.initialized = true
	return f.initializeErr
}

func (f *fakeInitPorts) InstallStatusHooks(ctx context.Context, hooks []domain.AgentStatusHook) error {
	f.calls = append(f.calls, "InstallStatusHooks")
	f.installed = append(f.installed, hooks)
	return f.installErr
}
