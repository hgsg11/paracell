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
  workspace: tmux
  notifications: tmux
templates:
  feat:
    commanderCell:
      name: workspace
      workspace:
        windows:
          - name: shell
            command: echo {{.Issue}}
      targets:
        app:
          source:
            path: .
            base: main
            branchPrefix: feat/
          container:
            environment:
              B: two
              A: one
            files:
              /workspace: .
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
	resolved, err := got.Resolve("feat", domain.NewTemplateVars("42", "42", "", ""))
	if err != nil {
		t.Fatal(err)
	}
	if len(resolved.Commander.Targets) != 1 || resolved.Commander.Targets[0].Name != "app" || resolved.Commander.Targets[0].Container.Mode != domain.Target {
		t.Fatalf("targets = %#v", resolved.Commander.Targets)
	}
	if resolved.Commander.Targets[0].Container.Environments[0].Name != "A" || resolved.Commander.Targets[0].Container.Mounts[0].TargetPath != "/workspace" {
		t.Fatalf("container = %#v", resolved.Commander.Targets[0].Container)
	}
	if resolved.Commander.Workspace.Windows[0].Command != "echo 42" {
		t.Fatalf("workspace = %#v, err = %v", resolved.Commander.Workspace, err)
	}
}

func TestYAMLConfigは未解決Templateを返す(t *testing.T) {
	path := filepath.Join(t.TempDir(), "paracell.yaml")
	data := []byte(`project:
  name: sample
providers:
  source: git
  workspace: tmux
templates:
  feat:
    extends: missing
    commanderCell:
      name: workspace
      workspace:
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
	if got.Templates[0].Extends != "missing" || got.Templates[0].Commander.Workspace.Windows[0].Command != "echo {{.Command}}" {
		t.Fatalf("template was resolved during load: %#v", got.Templates[0])
	}
	if _, err := got.Resolve("feat", domain.NewTemplateVars("", "", "sample", "work")); err == nil {
		t.Fatal("domain resolution must reject the missing parent")
	}
}

func TestYAMLConfigはCommanderCell構成を読み込み解決する(t *testing.T) {
	path := filepath.Join(t.TempDir(), "paracell.yaml")
	data := []byte(`project:
  name: sample
providers:
  source: git
  container: docker
  workspace: tmux
templates:
  multi:
    commanderCell:
      name: workspace
      workspace:
        windows:
          - name: agent
            command: 'codex {{.Command}}'
      targets:
        api:
          source:
            path: services/api
            base: origin/main
            branchPrefix: feat/api-
          container:
            environment:
              PORT: '8080'
          dependencies: [postgres]
        docs:
          source:
            path: docs
      dependencies:
        postgres:
          container: {}
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := (YAMLConfigAdapter{Path: path}).Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := got.Resolve("multi", domain.NewTemplateVars("113", "113", "sample", "make test"))
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Commander == nil || resolved.Commander.Name != "workspace" || len(resolved.Commander.Targets) != 2 || len(resolved.Commander.Dependencies) != 1 {
		t.Fatalf("resolved commander structure = %#v", resolved.Commander)
	}
	if resolved.Commander.Workspace.Windows[0].Command != "codex make test" {
		t.Fatalf("workspace command = %q", resolved.Commander.Workspace.Windows[0].Command)
	}
	api := resolved.Commander.Targets[0]
	if api.Name != "api" || api.Source == nil || api.Source.Path != "services/api" || api.Container == nil || api.Container.Environments[0].Value != "8080" {
		t.Fatalf("resolved api target = %#v", api)
	}
}

func TestYAMLConfigはCommanderCellの不正な依存参照を拒否する(t *testing.T) {
	path := filepath.Join(t.TempDir(), "paracell.yaml")
	data := []byte(`project: {name: sample}
providers: {source: git, workspace: tmux}
templates:
  multi:
    commanderCell:
      name: workspace
      workspace: {windows: []}
      targets:
        api:
          source: {path: .}
          dependencies: [missing]
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := (YAMLConfigAdapter{Path: path}).Load(context.Background()); err == nil {
		t.Fatal("expected unknown dependency reference to be rejected")
	}
}

func TestYAMLConfigはCommanderCell構成をSaveConfig後も維持する(t *testing.T) {
	path := filepath.Join(t.TempDir(), "paracell.yaml")
	input := []byte(`project: {name: sample}
providers: {source: git, workspace: tmux}
templates:
  multi:
    commanderCell:
      name: workspace
      workspace: {windows: []}
      targets:
        api: {source: {path: services/api}}
`)
	if err := os.WriteFile(path, input, 0o600); err != nil {
		t.Fatal(err)
	}
	adapter := YAMLConfigAdapter{Path: path}
	cfg, err := adapter.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := adapter.SaveConfig(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	reloaded, err := adapter.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := reloaded.Resolve("multi", domain.NewTemplateVars("113", "113", "sample", ""))
	if err != nil || resolved.Commander == nil || len(resolved.Commander.Targets) != 1 || resolved.Commander.Targets[0].Source == nil {
		t.Fatalf("commander template did not survive config save/load: %#v, %v", resolved, err)
	}
}
