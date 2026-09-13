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
	got, err := (YAMLConfigAdapter{Path: path}).Load(context.Background(), &domain.TemplateVars{Issue: "42"})
	if err != nil {
		t.Fatal(err)
	}
	if got.ProjectName != "sample" || got.GetContainerDriverType() != domain.Docker {
		t.Fatalf("templates = %#v", got)
	}
	containers, err := got.GetContainerTemplates("feat")
	if err != nil {
		t.Fatal(err)
	}
	if len(containers) != 1 || containers[0].Name != "app" || containers[0].Mode != domain.Target {
		t.Fatalf("containers = %#v", containers)
	}
	if containers[0].Environments[0].Name != "A" || containers[0].Mounts[0].TargetPath != "/workspace" {
		t.Fatalf("container = %#v", containers[0])
	}
	session, err := got.GetSessionTemplate("feat")
	if err != nil || session.Windows[0].Command != "echo 42" {
		t.Fatalf("session = %#v, err = %v", session, err)
	}
}
