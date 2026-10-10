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
prefixes:
  feat: feature/
  urgent: urgent/
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
        containers:
          web:
            environment:
              B: two
              A: one
            files:
              /workspace: .
          worker: {}
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
	if got.Prefixes[domain.FeatPrefix] != "feature/" || got.Prefixes[domain.FixPrefix] != "fix/" || got.Prefixes["urgent"] != "urgent/" {
		t.Fatalf("prefixes = %#v", got.Prefixes)
	}
	resolved, err := got.Resolve("feat", domain.NewTemplateVars("42", "42", "", ""))
	if err != nil {
		t.Fatal(err)
	}
	if len(resolved.Targets) != 1 || resolved.Targets[0].Name != "app" || len(resolved.Targets[0].Containers) != 2 || resolved.Targets[0].Containers[0].Mode != domain.Target {
		t.Fatalf("targets = %#v", resolved.Targets)
	}
	if resolved.Targets[0].Containers[0].Environments[0].Name != "A" || resolved.Targets[0].Containers[0].Mounts[0].TargetPath != "/workspace" {
		t.Fatalf("container = %#v", resolved.Targets[0].Containers)
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
        containers:
          app:
            environment:
              PORT: '8080'
          worker: {}
      docs:
        source:
          path: docs
    dependencies: [postgres]
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
	if resolved.Commander == nil || resolved.Commander.Name != "workspace" || len(resolved.Targets) != 2 || len(resolved.Dependencies) != 1 {
		t.Fatalf("resolved template structure = %#v", resolved)
	}
	if resolved.Commander.Workspace.Windows[0].Command != "codex make test" {
		t.Fatalf("workspace command = %q", resolved.Commander.Workspace.Windows[0].Command)
	}
	api := resolved.Targets[0]
	if api.Name != "api" || api.Source == nil || api.Source.Path != "services/api" || len(api.Containers) != 2 || api.Containers[0].Environments[0].Value != "8080" || resolved.Dependencies[0].Name != "postgres" {
		t.Fatalf("resolved api target = %#v", api)
	}
}

func TestYAMLConfigはTarget依存関係を設定させない(t *testing.T) {
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
		t.Fatal("TargetCell dependency link was accepted")
	}
}

func TestYAMLConfigはDependencyに名前以外の設定を許さない(t *testing.T) {
	path := filepath.Join(t.TempDir(), "paracell.yaml")
	data := []byte(`project: {name: sample}
providers: {source: git, workspace: tmux}
templates:
  feature:
    commanderCell: {name: workspace, workspace: {windows: []}}
    dependencies:
      - name: database
        container: {}
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewYAMLConfigAdapter(path).Load(context.Background()); err == nil {
		t.Fatal("DependencyCell configuration object was accepted")
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
	if err != nil || resolved.Commander == nil || len(resolved.Targets) != 1 || resolved.Targets[0].Source == nil {
		t.Fatalf("commander template did not survive config save/load: %#v, %v", resolved, err)
	}
}

func TestYAMLConfigNotificationDrivers(t *testing.T) {
	for _, value := range []string{"terminal-notifier", "tmux", "none", "", "invalid"} {
		t.Run(value, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "paracell.yaml")
			data := "project: {name: sample}\nproviders:\n  source: git\n  workspace: tmux\n  notifications: " + value + "\ntemplates: {}\n"
			if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
				t.Fatal(err)
			}
			cfg, err := NewYAMLConfigAdapter(path).Load(context.Background())
			if value == "invalid" {
				if err == nil {
					t.Fatal("invalid notification driver accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			want := value
			if want == "" {
				want = "none"
			}
			if string(cfg.NotificationDriverType) != want {
				t.Fatalf("notification driver = %q, want %q", cfg.NotificationDriverType, want)
			}
		})
	}
}

func TestParacellYAMLUsesPeerCellDefinitions(t *testing.T) {
	path := filepath.Join("..", "..", "..", "paracell.yaml")
	cfg, err := NewYAMLConfigAdapter(path).Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	names, err := cfg.SelectableNames()
	if err != nil {
		t.Fatal(err)
	}
	var resolved domain.ResolvedTemplate
	found := false
	for _, name := range names {
		candidate, err := cfg.Resolve(name, domain.NewTemplateVars("126", "126", "paracell", "test"))
		if err != nil {
			t.Fatal(err)
		}
		if len(candidate.Targets) == 1 && candidate.Targets[0].Name == "repository" && len(candidate.Dependencies) == 0 {
			resolved = candidate
			found = true
			break
		}
	}
	if !found {
		t.Fatal("no template defines the repository target without dependencies")
	}
	if resolved.Commander == nil || len(resolved.Targets) != 1 || resolved.Targets[0].Name != "repository" || len(resolved.Dependencies) != 0 {
		t.Fatalf("resolved peer template = %#v", resolved)
	}
}
