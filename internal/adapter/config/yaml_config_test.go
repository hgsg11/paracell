package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/shige1114/paradev/internal/domain"
)

func TestYAML設定からProjectとTemplateを読み込める(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".pdev.yml")
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
    containers:
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
	cfg, err := loader.Load(context.Background())

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
	if template.Session.Windows[0].Command != "nvim ." {
		t.Fatalf("session command = %q, want %q", template.Session.Windows[0].Command, "nvim .")
	}
}

func TestYAML設定を保存できる(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".pdev.yml")
	adapter := YAMLConfigAdapter{Path: configPath}

	err := adapter.SaveConfig(context.Background(), domain.Config{
		Project: domain.ProjectConfig{Name: "paradev"},
		Templates: map[string]domain.Template{
			"default": {
				Name: "default",
				Repository: domain.RepositoryTemplate{
					BranchPrefix: "feat/",
					Base:         "main",
				},
				Containers: domain.ContainerTemplate{
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
templates:
    default:
        repository:
            branchPrefix: feat/
            base: main
        containers:
            services:
                web:
                    sourceContainer: myapp-web
        session:
            windows: []
`
	if string(data) != want {
		t.Fatalf("保存内容 =\n%s\nwant =\n%s", string(data), want)
	}
	info, err := os.Stat(filepath.Join(dir, ".pdev"))
	if err != nil {
		t.Fatalf(".pdev directoryが作られていない: %v", err)
	}
	if !info.IsDir() {
		t.Fatal(".pdev がdirectoryではない")
	}
}
