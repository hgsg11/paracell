package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hgsg11/paracell/internal/domain"
)

func TestYAMLConfigは新しいTemplateDomainへ変換する(t *testing.T) {
	path := filepath.Join(t.TempDir(), "paracell.yaml")
	data := []byte(`project:
  name: sample
providers:
  source: git
  container: docker
  session: tmux
  notifications: tmux
templates:
  feat:
    repository:
      path: .
      base: main
      branchPrefix: feat/
    containers:
      services:
        app:
          mode: target
          environment:
            B: two
            A: one
          files:
            /workspace: .
    session:
      windows:
        - name: shell
          command: echo {{.Issue}}
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := (YAMLConfigAdapter{Path: path}).Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.ProjectName != "sample" || got.ContainerDriverType != domain.Docker {
		t.Fatalf("templates = %#v", got)
	}
	resolved, err := domain.ResolveTemplate(got, "feat", domain.NewTemplateVars("42", "42", "", ""))
	if err != nil {
		t.Fatal(err)
	}
	if len(resolved.Containers) != 1 || resolved.Containers[0].Name != "app" || resolved.Containers[0].Mode != domain.Target {
		t.Fatalf("containers = %#v", resolved.Containers)
	}
	if resolved.Containers[0].Environments[0].Name != "A" || resolved.Containers[0].Mounts[0].TargetPath != "/workspace" {
		t.Fatalf("container = %#v", resolved.Containers[0])
	}
	if resolved.Session.Windows[0].Command != "echo 42" {
		t.Fatalf("session = %#v, err = %v", resolved.Session, err)
	}
}

func TestYAMLConfigは未解決Templateを返す(t *testing.T) {
	path := filepath.Join(t.TempDir(), "paracell.yaml")
	data := []byte(`project:
  name: sample
providers:
  source: git
  session: tmux
templates:
  feat:
    extends: missing
    session:
      windows:
        - name: shell
          command: echo {{.Command}}
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := NewYAMLConfigAdapter(path).Load(context.Background())
	if err != nil {
		t.Fatalf("Load resolved inheritance unexpectedly: %v", err)
	}
	if got.Templates[0].Extends != "missing" || got.Templates[0].Session.Windows[0].Command != "echo {{.Command}}" {
		t.Fatalf("template was resolved during load: %#v", got.Templates[0])
	}
	if _, err := domain.ResolveTemplate(got, "feat", domain.NewTemplateVars("", "", "sample", "work")); err == nil {
		t.Fatal("domain resolution must reject the missing parent")
	}
}
