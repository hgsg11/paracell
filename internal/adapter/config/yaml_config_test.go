package config

import (
	"context"
	"os"
	"path/filepath"
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
	cfg, err := loader.Load(context.Background(), nil)

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
	template := cfg.Templates["webapp"]
	if template.Name != "webapp" {
		t.Fatalf("template名 = %q, want %q", template.Name, "webapp")
	}
	if template.Containers.Services["web"].SourceContainer != "myapp-web" {
		t.Fatalf("webのsourceContainer = %q, want %q", template.Containers.Services["web"].SourceContainer, "myapp-web")
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

func TestYAML設定はCommandをSessionCommandへ展開する(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "paracell.yaml")
	content := []byte(`project:
  name: myapp
providers:
  source: git
  session: tmux
templates:
  default:
    repository:
      base: main
    containers:
      services: {}
    session:
      windows:
        - name: agent
          command: codex {{.Command}}
`)
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		t.Fatalf("テスト用設定ファイルを書けなかった: %v", err)
	}

	cfg, err := (YAMLConfigAdapter{Path: configPath}).Load(context.Background(), &domain.TemplateVars{
		Issue:   "123",
		Name:    "123",
		Command: "review the API",
	})
	if err != nil {
		t.Fatalf("設定読み込みでエラーが返った: %v", err)
	}
	if got := cfg.Templates["default"].Session.Windows[0].Command; got != "codex review the API" {
		t.Fatalf("session command = %q, want %q", got, "codex review the API")
	}
}

func TestYAML設定を保存できる(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "paracell.yaml")
	adapter := YAMLConfigAdapter{Path: configPath}

	err := adapter.SaveConfig(context.Background(), domain.Config{
		Project: domain.ProjectConfig{Name: "paradev"},
		Providers: domain.ProviderConfig{
			Source:    "git",
			Container: "docker",
			Session:   "tmux",
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
						"web": {SourceContainer: "myapp-web"},
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
	if service.Database.CopyMode != "schema" {
		t.Fatalf("database.copyMode = %q, want %q", service.Database.CopyMode, "schema")
	}
	if len(service.Database.InitFiles) != 1 || service.Database.InitFiles[0] != "docker/mysql/init/001-users.sql" {
		t.Fatalf("database.initFiles = %#v, want [docker/mysql/init/001-users.sql]", service.Database.InitFiles)
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

func TestYAMLConfigは空でない不正なRepositoryBaseを拒否する(t *testing.T) {
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
      base: feature/other
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
		t.Fatal("不正なrepository.baseなのにエラーが返らなかった")
	}
	if err.Error() != `unsupported repository.base "feature/other" for template "default"` {
		t.Fatalf("error = %q, want %q", err.Error(), `unsupported repository.base "feature/other" for template "default"`)
	}
}
