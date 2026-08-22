package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hgsg11/paracell/internal/domain"
)

func TestYAML設定からProjectとTemplateを読み込める(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "paracell.yaml")
	content := []byte(`project:
  name: myapp
providers:
  source: git
  container: docker
  session: tmux
  notifications: tmux
templates:
  webapp:
    repository:
      branchPrefix: feat/
      base: main
    files:
      - .env
      - apps/web/.env.local
    containers:
      network: isolated
      services:
        web:
          sourceContainer: myapp-web
          environment:
            CELL_ISSUE: "{{.issue}}"
            CELL_NAME: "{{.name}}"
            PROJECT_NAME: "{{.project}}"
            EXPLICIT_EMPTY: ""
        db:
          sourceContainer: myapp-db
    session:
      windows:
        - name: editor
          command: nvim .
`)
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		t.Fatalf("テスト用設定ファイルを書けなかった: %v", err)
	}

	loader := YAMLConfigAdapter{Path: configPath}
	cfg, err := loader.Load(context.Background(), &domain.TemplateVars{Issue: "issue-123", Name: "cell-123"})

	if err != nil {
		t.Fatalf("設定読み込みでエラーが返った: %v", err)
	}
	if cfg.Project.Name != "myapp" {
		t.Fatalf("project.name = %q, want %q", cfg.Project.Name, "myapp")
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
	template := cfg.Templates["webapp"]
	if template.Name != "webapp" {
		t.Fatalf("template名 = %q, want %q", template.Name, "webapp")
	}
	if template.Containers.Services["web"].SourceContainer != "myapp-web" {
		t.Fatalf("webのsourceContainer = %q, want %q", template.Containers.Services["web"].SourceContainer, "myapp-web")
	}
	wantEnvironment := map[string]string{
		"CELL_ISSUE":     "issue-123",
		"CELL_NAME":      "cell-123",
		"PROJECT_NAME":   "myapp",
		"EXPLICIT_EMPTY": "",
	}
	if got := template.Containers.Services["web"].Environment; !reflect.DeepEqual(got, wantEnvironment) {
		t.Fatalf("web environment = %#v, want %#v", got, wantEnvironment)
	}
	if got := template.Containers.Services["db"].Environment; got != nil {
		t.Fatalf("db environment = %#v, want nil", got)
	}
	if template.Containers.Network != "isolated" {
		t.Fatalf("containers.network = %q, want %q", template.Containers.Network, "isolated")
	}
	if len(template.Files) != 2 || template.Files[0] != ".env" || template.Files[1] != "apps/web/.env.local" {
		t.Fatalf("files = %#v, want .env and apps/web/.env.local", template.Files)
	}
	if template.Session.Windows[0].Command != "nvim ." {
		t.Fatalf("session command = %q, want %q", template.Session.Windows[0].Command, "nvim .")
	}
}

func TestYAML設定を保存できる(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "paracell.yaml")
	adapter := YAMLConfigAdapter{Path: configPath}

	err := adapter.SaveConfig(context.Background(), domain.Config{
		Project: domain.ProjectConfig{Name: "paradev"},
		Providers: domain.ProviderConfig{
			Source:        "git",
			Container:     "docker",
			Session:       "tmux",
			Notifications: "tmux",
		},
		Templates: map[string]domain.Template{
			"default": {
				Name: "default",
				Repository: domain.RepositoryTemplate{
					BranchPrefix: "feat/",
					Base:         "main",
				},
				Files: []string{".env"},
				Containers: domain.ContainerTemplate{
					Network: "shared",
					Services: map[string]domain.ContainerServiceTemplate{
						"web": {
							SourceContainer: "myapp-web",
							Environment:     map[string]string{"APP_ENV": "cell"},
						},
					},
				},
				Session: domain.SessionTemplate{Windows: []domain.SessionWindowTemplate{}},
			},
		},
	})

	if err != nil {
		t.Fatalf("設定保存でエラーが返った: %v", err)
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("保存した設定を読めなかった: %v", err)
	}
	want := `project:
    name: paradev
providers:
    source: git
    container: docker
    session: tmux
    notifications: tmux
templates:
    default:
        repository:
            branchPrefix: feat/
            base: main
        files:
            - .env
        containers:
            network: shared
            services:
                web:
                    sourceContainer: myapp-web
                    environment:
                        APP_ENV: cell
        session:
            windows: []
`
	if string(data) != want {
		t.Fatalf("保存内容 =\n%s\nwant =\n%s", string(data), want)
	}
	info, err := os.Stat(filepath.Join(dir, ".paracell"))
	if err != nil {
		t.Fatalf(".paracell directoryが作られていない: %v", err)
	}
	if !info.IsDir() {
		t.Fatal(".paracell がdirectoryではない")
	}
}

func TestYAMLConfigは不正なEnvironmentTemplateを拒否する(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "paracell.yaml")
	content := []byte(`project:
  name: myapp
providers:
  source: git
  container: docker
  session: tmux
templates:
  default:
    repository:
      branchPrefix: feat/
      base: main
    containers:
      services:
        web:
          sourceContainer: myapp-web
          environment:
            BROKEN: "{{.issue"
    session:
      windows: []
`)
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		t.Fatalf("テスト用設定ファイルを書けなかった: %v", err)
	}

	_, err := (YAMLConfigAdapter{Path: configPath}).Load(context.Background(), &domain.TemplateVars{Issue: "123", Name: "123"})
	if err == nil {
		t.Fatal("不正なenvironment templateなのにエラーが返らなかった")
	}
	if !strings.Contains(err.Error(), `render environment "BROKEN" for service "web"`) {
		t.Fatalf("error = %q, want environment context", err)
	}
}

func TestYAMLConfigは未知のEnvironmentTemplate変数を拒否する(t *testing.T) {
	for _, unknown := range []string{"unknown", "Command"} {
		t.Run(unknown, func(t *testing.T) {
			dir := t.TempDir()
			configPath := filepath.Join(dir, "paracell.yaml")
			content := []byte(strings.ReplaceAll(`project:
  name: myapp
providers:
  source: git
  container: docker
  session: tmux
templates:
  default:
    repository:
      branchPrefix: feat/
      base: main
    containers:
      services:
        web:
          sourceContainer: myapp-web
          environment:
            UNKNOWN: "{{.UNKNOWN}}"
    session:
      windows: []
`, "UNKNOWN", unknown))
			if err := os.WriteFile(configPath, content, 0o644); err != nil {
				t.Fatalf("テスト用設定ファイルを書けなかった: %v", err)
			}

			_, err := (YAMLConfigAdapter{Path: configPath}).Load(context.Background(), &domain.TemplateVars{Issue: "123", Name: "123"})
			if err == nil {
				t.Fatal("未知のenvironment template変数なのにエラーが返らなかった")
			}
			if !strings.Contains(err.Error(), fmt.Sprintf(`map has no entry for key %q`, unknown)) {
				t.Fatalf("error = %q, want unknown variable error", err)
			}
		})
	}
}

func TestYAML設定を保存するとき空のContainer設定を省略する(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "paracell.yaml")
	adapter := YAMLConfigAdapter{Path: configPath}

	err := adapter.SaveConfig(context.Background(), domain.Config{
		Project: domain.ProjectConfig{Name: ""},
		Providers: domain.ProviderConfig{
			Source:        "git",
			Session:       "tmux",
			Notifications: "tmux",
		},
		Templates: map[string]domain.Template{
			"feat": {
				Name: "feat",
				Repository: domain.RepositoryTemplate{
					BranchPrefix: "feat/",
					Base:         "main",
				},
				Session: domain.SessionTemplate{Windows: []domain.SessionWindowTemplate{}},
			},
		},
	})
	if err != nil {
		t.Fatalf("設定保存でエラーが返った: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("保存した設定を読めなかった: %v", err)
	}
	want := `project:
    name: ""
providers:
    source: git
    session: tmux
    notifications: tmux
templates:
    feat:
        repository:
            branchPrefix: feat/
            base: main
        session:
            windows: []
`
	if string(data) != want {
		t.Fatalf("保存内容 =\n%s\nwant =\n%s", string(data), want)
	}
}

func TestYAML設定はProvidersがない場合に失敗する(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "paracell.yaml")
	content := []byte(`project:
  name: myapp
templates:
  webapp:
    repository:
      branchPrefix: feat/
      base: main
    containers:
      services: {}
    session:
      windows: []
`)
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		t.Fatalf("テスト用設定ファイルを書けなかった: %v", err)
	}

	loader := YAMLConfigAdapter{Path: configPath}
	_, err := loader.Load(context.Background(), nil)

	if err == nil {
		t.Fatal("providersがないのにエラーが返らなかった")
	}
	if err.Error() != "providers.source is required" {
		t.Fatalf("error = %q, want %q", err.Error(), "providers.source is required")
	}
}

func TestYAML設定は未対応Providerの場合に失敗する(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "paracell.yaml")
	content := []byte(`project:
  name: myapp
providers:
  source: svn
  container: docker
  session: tmux
templates:
  webapp:
    repository:
      branchPrefix: feat/
      base: main
    containers:
      services: {}
    session:
      windows: []
`)
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		t.Fatalf("テスト用設定ファイルを書けなかった: %v", err)
	}

	loader := YAMLConfigAdapter{Path: configPath}
	_, err := loader.Load(context.Background(), nil)

	if err == nil {
		t.Fatal("未対応providerなのにエラーが返らなかった")
	}
	if err.Error() != `unsupported providers.source "svn"` {
		t.Fatalf("error = %q, want %q", err.Error(), `unsupported providers.source "svn"`)
	}
}

func TestYAML設定は未対応NotificationProviderの場合に失敗する(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "paracell.yaml")
	content := []byte(`project:
  name: myapp
providers:
  source: git
  container: docker
  session: tmux
  notifications: slack
templates:
  webapp:
    repository:
      branchPrefix: feat/
      base: main
    containers:
      services: {}
    session:
      windows: []
`)
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		t.Fatalf("テスト用設定ファイルを書けなかった: %v", err)
	}

	loader := YAMLConfigAdapter{Path: configPath}
	_, err := loader.Load(context.Background(), nil)

	if err == nil {
		t.Fatal("未対応notification providerなのにエラーが返らなかった")
	}
	if err.Error() != `unsupported providers.notifications "slack"` {
		t.Fatalf("error = %q, want %q", err.Error(), `unsupported providers.notifications "slack"`)
	}
}

func TestYAML設定はContainerProviderがない場合も読み込める(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "paracell.yaml")
	content := []byte(`project:
  name: myapp
providers:
  source: git
  session: tmux
templates:
  webapp:
    repository:
      branchPrefix: feat/
      base: main
    containers:
      services: {}
    session:
      windows: []
`)
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		t.Fatalf("テスト用設定ファイルを書けなかった: %v", err)
	}

	loader := YAMLConfigAdapter{Path: configPath}
	cfg, err := loader.Load(context.Background(), nil)

	if err != nil {
		t.Fatalf("設定読み込みでエラーが返った: %v", err)
	}
	if cfg.Providers.Container != "" {
		t.Fatalf("providers.container = %q, want empty", cfg.Providers.Container)
	}
}

func TestYAML設定はContainerProviderが空文字でも読み込める(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "paracell.yaml")
	content := []byte(`project:
  name: myapp
providers:
  source: git
  container: ""
  session: tmux
templates:
  webapp:
    repository:
      branchPrefix: feat/
      base: main
    containers:
      services: {}
    session:
      windows: []
`)
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		t.Fatalf("テスト用設定ファイルを書けなかった: %v", err)
	}

	loader := YAMLConfigAdapter{Path: configPath}
	cfg, err := loader.Load(context.Background(), nil)

	if err != nil {
		t.Fatalf("設定読み込みでエラーが返った: %v", err)
	}
	if cfg.Providers.Container != "" {
		t.Fatalf("providers.container = %q, want empty", cfg.Providers.Container)
	}
}

func TestYAML設定は未対応ContainerProviderの場合に失敗する(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "paracell.yaml")
	content := []byte(`project:
  name: myapp
providers:
  source: git
  container: podman
  session: tmux
templates:
  webapp:
    repository:
      branchPrefix: feat/
      base: main
    containers:
      services: {}
    session:
      windows: []
`)
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		t.Fatalf("テスト用設定ファイルを書けなかった: %v", err)
	}

	loader := YAMLConfigAdapter{Path: configPath}
	_, err := loader.Load(context.Background(), nil)

	if err == nil {
		t.Fatal("未対応container providerなのにエラーが返らなかった")
	}
	if err.Error() != `unsupported providers.container "podman"` {
		t.Fatalf("error = %q, want %q", err.Error(), `unsupported providers.container "podman"`)
	}
}

func TestYAMLConfigはDBCopy設定を読み込む(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "paracell.yaml")
	content := []byte(`project:
  name: myapp
providers:
  source: git
  container: docker
  session: tmux
templates:
  default:
    repository:
      branchPrefix: feat/
      base: main
    containers:
      services:
        db:
          sourceContainer: myapp-db
          volumeMode: copy
          database:
            system: mysql
            copyMode: schema
            initFiles:
              - docker/mysql/init/001-users.sql
    session:
      windows: []
`)
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		t.Fatalf("テスト用設定ファイルを書けなかった: %v", err)
	}

	cfg, err := (YAMLConfigAdapter{Path: configPath}).Load(context.Background(), nil)
	if err != nil {
		t.Fatalf("設定読み込みでエラーが返った: %v", err)
	}

	service := cfg.Templates["default"].Containers.Services["db"]
	if service.Database == nil {
		t.Fatal("database = nil, want non-nil")
	}
	if service.Database.System != "mysql" {
		t.Fatalf("database.system = %q, want %q", service.Database.System, "mysql")
	}
	if service.Database.Mode != domain.DatabaseModeCopy {
		t.Fatalf("database.mode = %q, want %q", service.Database.Mode, domain.DatabaseModeCopy)
	}
	if service.Database.CopyMode != "schema" {
		t.Fatalf("database.copyMode = %q, want %q", service.Database.CopyMode, "schema")
	}
	if len(service.Database.InitFiles) != 1 || service.Database.InitFiles[0] != "docker/mysql/init/001-users.sql" {
		t.Fatalf("database.initFiles = %#v, want [docker/mysql/init/001-users.sql]", service.Database.InitFiles)
	}
}

func TestYAMLConfigはSharedDB設定を読み込む(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "paracell.yaml")
	content := []byte(`project:
  name: myapp
providers:
  source: git
  container: docker
  session: tmux
templates:
  default:
    repository:
      branchPrefix: feat/
      base: main
    containers:
      network: isolated
      services:
        db:
          sourceContainer: myapp-db
          database:
            mode: shared
    session:
      windows: []
`)
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := (YAMLConfigAdapter{Path: configPath}).Load(context.Background(), nil)
	if err != nil {
		t.Fatalf("設定読み込みでエラーが返った: %v", err)
	}
	if got := cfg.Templates["default"].Containers.Services["db"].Database.Mode; got != domain.DatabaseModeShared {
		t.Fatalf("database.mode = %q, want %q", got, domain.DatabaseModeShared)
	}
}

func TestYAMLConfigはDataCopyを設定読み込み時に拒否する(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "paracell.yaml")
	content := []byte(`project:
  name: myapp
providers:
  source: git
  container: docker
  session: tmux
templates:
  default:
    repository:
      branchPrefix: feat/
      base: main
    containers:
      network: isolated
      services:
        db:
          sourceContainer: myapp-db
          volumeMode: copy
          database:
            mode: copy
            system: mysql
            copyMode: data
    session:
      windows: []
`)
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := (YAMLConfigAdapter{Path: configPath}).Load(context.Background(), nil)
	if err == nil || err.Error() != `copyMode "data" is not implemented for service "db"` {
		t.Fatalf("error = %v, want data copy validation error", err)
	}
}

func TestYAMLConfigはSharedDBとVolumeCopyの併用を拒否する(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "paracell.yaml")
	content := []byte(`project:
  name: myapp
providers:
  source: git
  container: docker
  session: tmux
templates:
  default:
    repository:
      branchPrefix: feat/
      base: main
    containers:
      network: isolated
      services:
        db:
          sourceContainer: myapp-db
          volumeMode: copy
          database:
            mode: shared
    session:
      windows: []
`)
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := (YAMLConfigAdapter{Path: configPath}).Load(context.Background(), nil)
	if err == nil || err.Error() != `database mode "shared" for service "db" does not support volumeMode` {
		t.Fatalf("error = %v, want shared volumeMode validation error", err)
	}
}

func TestYAMLConfigは未対応databaseSystemを拒否する(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "paracell.yaml")
	content := []byte(`project:
  name: myapp
providers:
  source: git
  container: docker
  session: tmux
templates:
  default:
    repository:
      branchPrefix: feat/
      base: main
    containers:
      services:
        db:
          sourceContainer: myapp-db
          volumeMode: copy
          database:
            system: postgres
    session:
      windows: []
`)
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		t.Fatalf("テスト用設定ファイルを書けなかった: %v", err)
	}

	_, err := (YAMLConfigAdapter{Path: configPath}).Load(context.Background(), nil)
	if err == nil {
		t.Fatal("未対応databaseSystemなのにエラーが返らなかった")
	}
	if err.Error() != `unsupported databaseSystem "postgres" for service "db"` {
		t.Fatalf("error = %q, want %q", err.Error(), `unsupported databaseSystem "postgres" for service "db"`)
	}
}

func TestYAMLConfigは未対応copyModeを拒否する(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "paracell.yaml")
	content := []byte(`project:
  name: myapp
providers:
  source: git
  container: docker
  session: tmux
templates:
  default:
    repository:
      branchPrefix: feat/
      base: main
    containers:
      services:
        db:
          sourceContainer: myapp-db
          volumeMode: copy
          database:
            system: mysql
            copyMode: nonsense
    session:
      windows: []
`)
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		t.Fatalf("テスト用設定ファイルを書けなかった: %v", err)
	}

	_, err := (YAMLConfigAdapter{Path: configPath}).Load(context.Background(), nil)
	if err == nil {
		t.Fatal("未対応copyModeなのにエラーが返らなかった")
	}
	if err.Error() != `unsupported copyMode "nonsense" for service "db"` {
		t.Fatalf("error = %q, want %q", err.Error(), `unsupported copyMode "nonsense" for service "db"`)
	}
}

func TestYAMLConfigはInitFilesの親ディレクトリ脱出を拒否する(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "paracell.yaml")
	content := []byte(`project:
  name: myapp
providers:
  source: git
  container: docker
  session: tmux
templates:
  default:
    repository:
      branchPrefix: feat/
      base: main
    containers:
      services:
        db:
          sourceContainer: myapp-db
          volumeMode: copy
          database:
            system: mysql
            copyMode: schema
            initFiles:
              - ../secrets.sql
    session:
      windows: []
`)
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		t.Fatalf("テスト用設定ファイルを書けなかった: %v", err)
	}

	_, err := (YAMLConfigAdapter{Path: configPath}).Load(context.Background(), nil)
	if err == nil {
		t.Fatal("不正なinitFiles pathなのにエラーが返らなかった")
	}
	if err.Error() != `initFiles path "../secrets.sql" for service "db" must stay within project root` {
		t.Fatalf("error = %q, want %q", err.Error(), `initFiles path "../secrets.sql" for service "db" must stay within project root`)
	}
}

func TestYAMLConfigはDatabase設定をdb以外のRoleで拒否する(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "paracell.yaml")
	content := []byte(`project:
  name: myapp
providers:
  source: git
  container: docker
  session: tmux
templates:
  default:
    repository:
      branchPrefix: feat/
      base: main
    containers:
      services:
        web:
          sourceContainer: myapp-web
          database:
            system: mysql
            copyMode: schema
    session:
      windows: []
`)
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		t.Fatalf("テスト用設定ファイルを書けなかった: %v", err)
	}

	_, err := (YAMLConfigAdapter{Path: configPath}).Load(context.Background(), nil)
	if err == nil {
		t.Fatal("db以外のroleなのにdatabase設定が受理された")
	}
	if err.Error() != `database config is only supported for service "db"` {
		t.Fatalf("error = %q, want %q", err.Error(), `database config is only supported for service "db"`)
	}
}

func TestYAMLConfigはVolumeMode設定を読み込む(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "paracell.yaml")
	content := []byte(`project:
  name: myapp
providers:
  source: git
  container: docker
  session: tmux
templates:
  default:
    repository:
      branchPrefix: feat/
      base: main
    containers:
      services:
        web:
          sourceContainer: myapp-web
          volumeMode: copy
    session:
      windows: []
`)
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		t.Fatalf("テスト用設定ファイルを書けなかった: %v", err)
	}

	cfg, err := (YAMLConfigAdapter{Path: configPath}).Load(context.Background(), nil)
	if err != nil {
		t.Fatalf("設定読み込みでエラーが返った: %v", err)
	}
	if got := cfg.Templates["default"].Containers.Services["web"].VolumeMode; got != "copy" {
		t.Fatalf("volumeMode = %q, want %q", got, "copy")
	}
}

func TestYAMLConfigは未対応volumeModeを拒否する(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "paracell.yaml")
	content := []byte(`project:
  name: myapp
providers:
  source: git
  container: docker
  session: tmux
templates:
  default:
    repository:
      branchPrefix: feat/
      base: main
    containers:
      services:
        web:
          sourceContainer: myapp-web
          volumeMode: nonsense
    session:
      windows: []
`)
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		t.Fatalf("テスト用設定ファイルを書けなかった: %v", err)
	}

	_, err := (YAMLConfigAdapter{Path: configPath}).Load(context.Background(), nil)
	if err == nil {
		t.Fatal("未対応volumeModeなのにエラーが返らなかった")
	}
	if err.Error() != `unsupported volumeMode "nonsense" for service "web"` {
		t.Fatalf("error = %q, want %q", err.Error(), `unsupported volumeMode "nonsense" for service "web"`)
	}
}

func TestYAMLConfigはdbServiceでVolumeModeを拒否する(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "paracell.yaml")
	content := []byte(`project:
  name: myapp
providers:
  source: git
  container: docker
  session: tmux
templates:
  default:
    repository:
      branchPrefix: feat/
      base: main
    containers:
      services:
        db:
          sourceContainer: myapp-db
          volumeMode: copy
    session:
      windows: []
`)
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		t.Fatalf("テスト用設定ファイルを書けなかった: %v", err)
	}

	_, err := (YAMLConfigAdapter{Path: configPath}).Load(context.Background(), nil)
	if err == nil {
		t.Fatal("db serviceのvolumeModeなのにエラーが返らなかった")
	}
	if err.Error() != `volumeMode is not supported for service "db"` {
		t.Fatalf("error = %q, want %q", err.Error(), `volumeMode is not supported for service "db"`)
	}
}

func TestYAMLConfigはRepositoryBaseCurrentを読み込む(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "paracell.yaml")
	content := []byte(`project:
  name: myapp
providers:
  source: git
  container: docker
  session: tmux
templates:
  default:
    repository:
      branchPrefix: feat/
      base: current
    containers:
      services: {}
    session:
      windows: []
`)
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		t.Fatalf("テスト用設定ファイルを書けなかった: %v", err)
	}

	cfg, err := (YAMLConfigAdapter{Path: configPath}).Load(context.Background(), nil)
	if err != nil {
		t.Fatalf("設定読み込みでエラーが返った: %v", err)
	}
	if got := cfg.Templates["default"].Repository.Base; got != "current" {
		t.Fatalf("repository.base = %q, want %q", got, "current")
	}
}

func TestYAMLConfigはRepositoryBranchModeReuseを読み込む(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "paracell.yaml")
	content := []byte(`project:
  name: myapp
providers:
  source: git
  container: docker
  session: tmux
templates:
  default:
    repository:
      branchPrefix: feat/
      base: main
      branchMode: reuse
    containers:
      services: {}
    session:
      windows: []
`)
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		t.Fatalf("テスト用設定ファイルを書けなかった: %v", err)
	}

	cfg, err := (YAMLConfigAdapter{Path: configPath}).Load(context.Background(), nil)
	if err != nil {
		t.Fatalf("設定読み込みでエラーが返った: %v", err)
	}
	if got := cfg.Templates["default"].Repository.BranchMode; got != "reuse" {
		t.Fatalf("repository.branchMode = %q, want %q", got, "reuse")
	}
}

func TestYAMLConfigは未対応ContainerNetworkを拒否する(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "paracell.yaml")
	content := []byte(`project:
  name: myapp
providers:
  source: git
  container: docker
  session: tmux
templates:
  default:
    repository:
      branchPrefix: feat/
      base: main
    containers:
      network: isolate
      services: {}
    session:
      windows: []
`)
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		t.Fatalf("テスト用設定ファイルを書けなかった: %v", err)
	}

	_, err := (YAMLConfigAdapter{Path: configPath}).Load(context.Background(), nil)
	if err == nil {
		t.Fatal("未対応containers.networkなのにエラーが返らなかった")
	}
	if err.Error() != `unsupported containers.network "isolate"` {
		t.Fatalf("error = %q, want %q", err.Error(), `unsupported containers.network "isolate"`)
	}
}

func TestYAMLConfigは不正なRepositoryBranchModeを拒否する(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "paracell.yaml")
	content := []byte(`project:
  name: myapp
providers:
  source: git
  container: docker
  session: tmux
templates:
  default:
    repository:
      branchPrefix: feat/
      base: main
      branchMode: overwrite
    containers:
      services: {}
    session:
      windows: []
`)
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		t.Fatalf("テスト用設定ファイルを書けなかった: %v", err)
	}

	_, err := (YAMLConfigAdapter{Path: configPath}).Load(context.Background(), nil)
	if err == nil {
		t.Fatal("不正なrepository.branchModeなのにエラーが返らなかった")
	}
	if err.Error() != `unsupported repository.branchMode "overwrite" for template "default"` {
		t.Fatalf("error = %q, want %q", err.Error(), `unsupported repository.branchMode "overwrite" for template "default"`)
	}
}

func TestYAMLConfigは任意のRepositoryBaseを読み込む(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "paracell.yaml")
	content := []byte(`project:
  name: myapp
providers:
  source: git
  container: docker
  session: tmux
templates:
  default:
    repository:
      branchPrefix: feat/
      base: feature/111
    containers:
      services: {}
    session:
      windows: []
`)
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		t.Fatalf("テスト用設定ファイルを書けなかった: %v", err)
	}

	cfg, err := (YAMLConfigAdapter{Path: configPath}).Load(context.Background(), nil)
	if err != nil {
		t.Fatalf("設定読み込みでエラーが返った: %v", err)
	}
	if got := cfg.Templates["default"].Repository.Base; got != "feature/111" {
		t.Fatalf("repository.base = %q, want %q", got, "feature/111")
	}
}

func TestYAMLConfigはTemplateを複数段継承して明示値で上書きする(t *testing.T) {
	cfg := loadInheritanceConfig(t, `
  base:
    abstract: true
    repository:
      branchPrefix: base/
      base: origin/main
      branchMode: create
    files: [.env, config/base.yaml]
    containers:
      network: isolated
      services:
        web:
          sourceContainer: myapp-web
          environment:
            CELL_NAME: "{{.name}}"
            PROJECT_NAME: "{{.project}}"
    session:
      windows:
        - name: agent
          command: 'codex "{{.Command}}" --issue {{.issue}} --name {{.name}}'
  intermediate:
    abstract: true
    extends: base
    repository:
      base: origin/release
  feat:
    extends: intermediate
    repository:
      branchPrefix: feat/
      branchMode: ""
`, &domain.TemplateVars{Issue: "77", Name: "cell-77", Command: "implement"})

	if _, ok := cfg.Templates["base"]; ok {
		t.Fatal("abstract template baseが選択可能なtemplateに含まれた")
	}
	if _, ok := cfg.Templates["intermediate"]; ok {
		t.Fatal("abstract template intermediateが選択可能なtemplateに含まれた")
	}
	if _, ok := cfg.AbstractTemplates["base"]; !ok {
		t.Fatal("baseがabstract templateとして記録されていない")
	}

	feat := cfg.Templates["feat"]
	if feat.Name != "feat" {
		t.Fatalf("template.Name = %q, want feat", feat.Name)
	}
	if feat.Repository.BranchPrefix != "feat/" {
		t.Fatalf("repository.branchPrefix = %q, want feat/", feat.Repository.BranchPrefix)
	}
	if feat.Repository.Base != "origin/release" {
		t.Fatalf("repository.base = %q, want origin/release", feat.Repository.Base)
	}
	if feat.Repository.BranchMode != "" {
		t.Fatalf("repository.branchMode = %q, want explicit empty", feat.Repository.BranchMode)
	}
	if !reflect.DeepEqual(feat.Files, []string{".env", "config/base.yaml"}) {
		t.Fatalf("files = %#v, want inherited files", feat.Files)
	}
	if feat.Containers.Network != domain.ContainerNetworkIsolated {
		t.Fatalf("containers.network = %q, want isolated", feat.Containers.Network)
	}
	wantEnvironment := map[string]string{"CELL_NAME": "cell-77", "PROJECT_NAME": "myapp"}
	if got := feat.Containers.Services["web"].Environment; !reflect.DeepEqual(got, wantEnvironment) {
		t.Fatalf("environment = %#v, want %#v", got, wantEnvironment)
	}
	wantCommand := "codex \"implement\" --issue 77 --name cell-77"
	if got := feat.Session.Windows[0].Command; got != wantCommand {
		t.Fatalf("session command = %q, want %q", got, wantCommand)
	}
}

func TestYAMLConfigはSliceとMapを全体置換できる(t *testing.T) {
	cfg := loadInheritanceConfig(t, `
  base:
    abstract: true
    repository:
      branchPrefix: base/
      base: main
    files: [.env, config/base.yaml]
    containers:
      services:
        web:
          sourceContainer: app-web
    session:
      windows:
        - name: inherited
          command: inherited
  replaced:
    extends: base
    files: [config/feat.yaml]
    containers:
      services:
        worker:
          sourceContainer: app-worker
    session:
      windows:
        - name: child
          command: child
  emptied:
    extends: base
    files: []
    containers:
      services: {}
    session:
      windows: []
`, nil)

	replaced := cfg.Templates["replaced"]
	if !reflect.DeepEqual(replaced.Files, []string{"config/feat.yaml"}) {
		t.Fatalf("replaced.files = %#v", replaced.Files)
	}
	if _, exists := replaced.Containers.Services["web"]; exists {
		t.Fatal("親のservices entryがdeep mergeされた")
	}
	if got := replaced.Containers.Services["worker"].SourceContainer; got != "app-worker" {
		t.Fatalf("worker.sourceContainer = %q, want app-worker", got)
	}
	if !reflect.DeepEqual(replaced.Session.Windows, []domain.SessionWindowTemplate{{Name: "child", Command: "child"}}) {
		t.Fatalf("session.windows = %#v, want childだけ", replaced.Session.Windows)
	}

	emptied := cfg.Templates["emptied"]
	if len(emptied.Files) != 0 {
		t.Fatalf("emptied.files = %#v, want empty", emptied.Files)
	}
	if emptied.Containers.Services == nil || len(emptied.Containers.Services) != 0 {
		t.Fatalf("emptied.services = %#v, want explicit empty map", emptied.Containers.Services)
	}
	if len(emptied.Session.Windows) != 0 {
		t.Fatalf("emptied.windows = %#v, want empty", emptied.Session.Windows)
	}
}

func TestYAMLConfigはTemplate継承の参照Errorを決定的に返す(t *testing.T) {
	tests := []struct {
		name      string
		templates string
		want      string
	}{
		{
			name: "unknown parent",
			templates: `
  feat:
    extends: base
`,
			want: `template "feat" extends unknown template "base"`,
		},
		{
			name: "self reference",
			templates: `
  self:
    extends: self
`,
			want: `template inheritance cycle: "self" -> "self"`,
		},
		{
			name: "cycle",
			templates: `
  c:
    extends: a
  b:
    extends: c
  a:
    extends: b
`,
			want: `template inheritance cycle: "a" -> "b" -> "c" -> "a"`,
		},
		{
			name: "sorted errors",
			templates: `
  z:
    extends: missing-z
  a:
    extends: missing-a
`,
			want: `template "a" extends unknown template "missing-a"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for range 10 {
				_, err := loadInheritanceConfigError(t, tt.templates, nil)
				if err == nil {
					t.Fatal("継承errorが返らなかった")
				}
				if err.Error() != tt.want {
					t.Fatalf("error = %q, want %q", err, tt.want)
				}
			}
		})
	}
}

func TestYAMLConfigは継承解決後の具体TemplateだけをValidationする(t *testing.T) {
	t.Run("abstractの値を子が修正できる", func(t *testing.T) {
		cfg := loadInheritanceConfig(t, `
  base:
    abstract: true
    repository:
      branchMode: invalid-until-overridden
  feat:
    extends: base
    repository:
      branchPrefix: feat/
      base: main
      branchMode: create
    containers:
      services: {}
    session:
      windows: []
`, nil)
		if got := cfg.Templates["feat"].Repository.BranchMode; got != "create" {
			t.Fatalf("branchMode = %q, want create", got)
		}
	})

	t.Run("継承した不正値は子の名前で拒否する", func(t *testing.T) {
		_, err := loadInheritanceConfigError(t, `
  base:
    abstract: true
    repository:
      branchPrefix: feat/
      base: main
      branchMode: overwrite
  feat:
    extends: base
`, nil)
		want := `unsupported repository.branchMode "overwrite" for template "feat"`
		if err == nil || err.Error() != want {
			t.Fatalf("error = %v, want %q", err, want)
		}
	})

	tests := []struct {
		name      string
		inherited string
		wantError string
	}{
		{
			name: "network",
			inherited: `
    containers:
      network: invalid
`,
			wantError: `unsupported containers.network "invalid"`,
		},
		{
			name: "service",
			inherited: `
    containers:
      services:
        z-service:
          volumeMode: invalid-z
        a-service:
          volumeMode: invalid-a
`,
			wantError: `unsupported volumeMode "invalid-a" for service "a-service"`,
		},
		{
			name: "database",
			inherited: `
    containers:
      services:
        db:
          volumeMode: copy
          database:
            system: postgres
`,
			wantError: `unsupported databaseSystem "postgres" for service "db"`,
		},
		{
			name: "path",
			inherited: `
    containers:
      services:
        db:
          volumeMode: copy
          database:
            system: mysql
            initFiles: [../outside.sql]
`,
			wantError: `initFiles path "../outside.sql" for service "db" must stay within project root`,
		},
	}
	for _, tt := range tests {
		t.Run("継承値/"+tt.name, func(t *testing.T) {
			templates := `
  base:
    abstract: true
` + tt.inherited + `  feat:
    extends: base
`
			for range 10 {
				_, err := loadInheritanceConfigError(t, templates, nil)
				if err == nil || err.Error() != tt.wantError {
					t.Fatalf("error = %v, want %q", err, tt.wantError)
				}
			}
		})
	}
}

func loadInheritanceConfig(t *testing.T, templates string, vars *domain.TemplateVars) domain.Config {
	t.Helper()
	cfg, err := loadInheritanceConfigError(t, templates, vars)
	if err != nil {
		t.Fatalf("設定読み込みでエラーが返った: %v", err)
	}
	return cfg
}

func loadInheritanceConfigError(t *testing.T, templates string, vars *domain.TemplateVars) (domain.Config, error) {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "paracell.yaml")
	content := `project:
  name: myapp
providers:
  source: git
  container: docker
  session: tmux
templates:` + templates
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("テスト用設定ファイルを書けなかった: %v", err)
	}
	return (YAMLConfigAdapter{Path: configPath}).Load(context.Background(), vars)
}
