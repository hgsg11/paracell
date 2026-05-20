package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestYAML設定からProjectとTemplateを読み込める(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".pdev.yml")
	content := []byte(`project:
  name: myapp
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
