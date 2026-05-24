# Provider Config Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add required provider configuration to `.pdev.yml` so `pdev` explicitly knows which source, container, and session implementations to use.

**Architecture:** `domain.Config` gains `Providers domain.ProviderConfig`. The YAML config adapter loads, saves, and validates providers. `InitProjectUseCase` emits the default `git/docker/tmux` provider config, and `internal/app` selects existing adapters through small provider factory functions.

**Tech Stack:** Go 1.26, standard library, `gopkg.in/yaml.v3`, existing Clean Architecture package layout.

---

## File Structure

- Modify `internal/domain/config.go`: add `ProviderConfig` to the domain config model.
- Modify `internal/adapter/config/yaml_config.go`: add YAML provider fields and validation.
- Modify `internal/adapter/config/yaml_config_test.go`: cover provider load/save and invalid config errors.
- Modify `internal/usecase/init_project.go`: include default provider config in `pdev init`.
- Modify `internal/usecase/init_test.go`: assert default providers from init.
- Create `internal/adapter/provider/adapters.go`: select source/container/session adapters from provider names.
- Create `internal/adapter/provider/adapters_test.go`: test supported and unsupported provider selection.
- Modify `internal/app/cli.go`: wire selected adapters for `create` and `remove`; keep `ls` independent of `.pdev.yml`.
- Modify `internal/app/cli_test.go`: add a regression test that `ls` does not require `.pdev.yml`.

## Task 1: Domain Provider Config

**Files:**
- Modify: `internal/domain/config.go`
- Modify: `internal/adapter/config/yaml_config_test.go`

- [ ] **Step 1: Write the failing model usage in config adapter tests**

In `internal/adapter/config/yaml_config_test.go`, update the YAML in `TestYAML設定からProjectとTemplateを読み込める` to include providers:

```go
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
```

Add this assertion after the project name assertion:

```go
	if cfg.Providers.Source != "git" {
		t.Fatalf("providers.source = %q, want %q", cfg.Providers.Source, "git")
	}
	if cfg.Providers.Container != "docker" {
		t.Fatalf("providers.container = %q, want %q", cfg.Providers.Container, "docker")
	}
	if cfg.Providers.Session != "tmux" {
		t.Fatalf("providers.session = %q, want %q", cfg.Providers.Session, "tmux")
	}
```

- [ ] **Step 2: Run the config adapter test to verify it fails**

Run:

```bash
go test ./internal/adapter/config -run TestYAML設定からProjectとTemplateを読み込める
```

Expected: FAIL with `cfg.Providers undefined`.

- [ ] **Step 3: Add provider config to the domain model**

Modify `internal/domain/config.go`:

```go
package domain

type ProjectConfig struct {
	Name string
}

type ProviderConfig struct {
	Source    string
	Container string
	Session   string
}

type Config struct {
	Project   ProjectConfig
	Providers ProviderConfig
	Templates map[string]Template
}
```

- [ ] **Step 4: Run the config adapter test to verify the failure moves to empty values**

Run:

```bash
go test ./internal/adapter/config -run TestYAML設定からProjectとTemplateを読み込める
```

Expected: FAIL because `providers.source`, `providers.container`, and `providers.session` are empty.

- [ ] **Step 5: Commit Task 1**

```bash
git add internal/domain/config.go internal/adapter/config/yaml_config_test.go
git commit -m "Add provider config model"
```

## Task 2: YAML Provider Load And Save

**Files:**
- Modify: `internal/adapter/config/yaml_config.go`
- Modify: `internal/adapter/config/yaml_config_test.go`

- [ ] **Step 1: Add failing YAML save expectations**

In `TestYAML設定を保存できる`, add providers to the config passed to `SaveConfig`:

```go
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
				Containers: domain.ContainerTemplate{
					Services: map[string]domain.ContainerServiceTemplate{
						"web": {SourceContainer: "myapp-web"},
					},
				},
				Session: domain.SessionTemplate{Windows: []domain.SessionWindowTemplate{}},
			},
		},
	})
```

Update the expected YAML:

```go
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
        containers:
            services:
                web:
                    sourceContainer: myapp-web
        session:
            windows: []
`
```

- [ ] **Step 2: Run YAML config tests to verify they fail**

Run:

```bash
go test ./internal/adapter/config
```

Expected: FAIL because providers are not loaded or saved.

- [ ] **Step 3: Add YAML provider fields and load/save mapping**

Modify `internal/adapter/config/yaml_config.go`:

```go
type yamlConfig struct {
	Project struct {
		Name string `yaml:"name"`
	} `yaml:"project"`
	Providers yamlProviders           `yaml:"providers"`
	Templates map[string]yamlTemplate `yaml:"templates"`
}

type yamlProviders struct {
	Source    string `yaml:"source"`
	Container string `yaml:"container"`
	Session   string `yaml:"session"`
}
```

In `Load`, set providers on the returned config:

```go
	return domain.Config{
		Project: domain.ProjectConfig{Name: raw.Project.Name},
		Providers: domain.ProviderConfig{
			Source:    raw.Providers.Source,
			Container: raw.Providers.Container,
			Session:   raw.Providers.Session,
		},
		Templates: templates,
	}, nil
```

In `SaveConfig`, set providers before marshaling:

```go
	raw := yamlConfig{
		Providers: yamlProviders{
			Source:    cfg.Providers.Source,
			Container: cfg.Providers.Container,
			Session:   cfg.Providers.Session,
		},
		Templates: make(map[string]yamlTemplate, len(cfg.Templates)),
	}
```

- [ ] **Step 4: Run YAML config tests to verify they pass**

Run:

```bash
go test ./internal/adapter/config
```

Expected: PASS.

- [ ] **Step 5: Commit Task 2**

```bash
git add internal/adapter/config/yaml_config.go internal/adapter/config/yaml_config_test.go
git commit -m "Load and save provider config"
```

## Task 3: YAML Provider Validation

**Files:**
- Modify: `internal/adapter/config/yaml_config.go`
- Modify: `internal/adapter/config/yaml_config_test.go`

- [ ] **Step 1: Write failing validation tests**

Add these tests to `internal/adapter/config/yaml_config_test.go`:

```go
func TestYAML設定はProvidersがない場合に失敗する(t *testing.T) {
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
      services: {}
    session:
      windows: []
`)
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		t.Fatalf("テスト用設定ファイルを書けなかった: %v", err)
	}

	loader := YAMLConfigAdapter{Path: configPath}
	_, err := loader.Load(context.Background())

	if err == nil {
		t.Fatal("providersがないのにエラーが返らなかった")
	}
	if err.Error() != "providers.source is required" {
		t.Fatalf("error = %q, want %q", err.Error(), "providers.source is required")
	}
}

func TestYAML設定は未対応Providerの場合に失敗する(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".pdev.yml")
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
	_, err := loader.Load(context.Background())

	if err == nil {
		t.Fatal("未対応providerなのにエラーが返らなかった")
	}
	if err.Error() != `unsupported providers.source "svn"` {
		t.Fatalf("error = %q, want %q", err.Error(), `unsupported providers.source "svn"`)
	}
}
```

- [ ] **Step 2: Run validation tests to verify they fail**

Run:

```bash
go test ./internal/adapter/config -run 'TestYAML設定は'
```

Expected: FAIL because invalid providers are accepted.

- [ ] **Step 3: Implement provider validation**

Add this helper to `internal/adapter/config/yaml_config.go`:

```go
func validateProviders(providers domain.ProviderConfig) error {
	if providers.Source == "" {
		return errors.New("providers.source is required")
	}
	if providers.Source != "git" {
		return fmt.Errorf("unsupported providers.source %q", providers.Source)
	}
	if providers.Container == "" {
		return errors.New("providers.container is required")
	}
	if providers.Container != "docker" {
		return fmt.Errorf("unsupported providers.container %q", providers.Container)
	}
	if providers.Session == "" {
		return errors.New("providers.session is required")
	}
	if providers.Session != "tmux" {
		return fmt.Errorf("unsupported providers.session %q", providers.Session)
	}
	return nil
}
```

Update imports to include `fmt`:

```go
import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/shige1114/paradev/internal/domain"
	"gopkg.in/yaml.v3"
)
```

In `Load`, construct providers and validate before returning:

```go
	providers := domain.ProviderConfig{
		Source:    raw.Providers.Source,
		Container: raw.Providers.Container,
		Session:   raw.Providers.Session,
	}
	if err := validateProviders(providers); err != nil {
		return domain.Config{}, err
	}
	return domain.Config{
		Project:   domain.ProjectConfig{Name: raw.Project.Name},
		Providers: providers,
		Templates: templates,
	}, nil
```

- [ ] **Step 4: Run validation tests to verify they pass**

Run:

```bash
go test ./internal/adapter/config -run 'TestYAML設定は'
```

Expected: PASS.

- [ ] **Step 5: Commit Task 3**

```bash
git add internal/adapter/config/yaml_config.go internal/adapter/config/yaml_config_test.go
git commit -m "Validate provider config"
```

## Task 4: Init Writes Default Providers

**Files:**
- Modify: `internal/usecase/init_project.go`
- Modify: `internal/usecase/init_test.go`

- [ ] **Step 1: Write failing init assertions**

In `internal/usecase/init_test.go`, add these assertions after the project name assertion in `TestInitは現在のProject情報から設定を作成して保存する`:

```go
	if cfg.Providers.Source != "git" {
		t.Fatalf("providers.source = %q, want %q", cfg.Providers.Source, "git")
	}
	if cfg.Providers.Container != "docker" {
		t.Fatalf("providers.container = %q, want %q", cfg.Providers.Container, "docker")
	}
	if cfg.Providers.Session != "tmux" {
		t.Fatalf("providers.session = %q, want %q", cfg.Providers.Session, "tmux")
	}
```

- [ ] **Step 2: Run init tests to verify they fail**

Run:

```bash
go test ./internal/usecase -run TestInitは現在のProject情報から設定を作成して保存する
```

Expected: FAIL because provider values are empty.

- [ ] **Step 3: Add default providers to init config**

Modify the `cfg := domain.Config{...}` literal in `internal/usecase/init_project.go`:

```go
	cfg := domain.Config{
		Project: domain.ProjectConfig{Name: ""},
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
				Containers: domain.ContainerTemplate{Services: map[string]domain.ContainerServiceTemplate{}},
				Session:    domain.SessionTemplate{Windows: []domain.SessionWindowTemplate{}},
			},
		},
	}
```

- [ ] **Step 4: Run init tests to verify they pass**

Run:

```bash
go test ./internal/usecase -run TestInitは現在のProject情報から設定を作成して保存する
```

Expected: PASS.

- [ ] **Step 5: Commit Task 4**

```bash
git add internal/usecase/init_project.go internal/usecase/init_test.go
git commit -m "Add default providers to init"
```

## Task 5: App Provider Selection

**Files:**
- Create: `internal/adapter/provider/adapters.go`
- Create: `internal/adapter/provider/adapters_test.go`
- Modify: `internal/app/cli.go`

- [ ] **Step 1: Write failing provider selection tests**

Create `internal/adapter/provider/adapters_test.go`:

```go
package provider

import (
	"testing"

	"github.com/shige1114/paradev/internal/domain"
)

func TestAdaptersは対応Providerを選択できる(t *testing.T) {
	adapters, err := NewAdapters(domain.ProviderConfig{
		Source:    "git",
		Container: "docker",
		Session:   "tmux",
	}, nil)

	if err != nil {
		t.Fatalf("provider adapter選択でエラーが返った: %v", err)
	}
	if adapters.Source == nil {
		t.Fatal("source adapter is nil")
	}
	if adapters.Containers == nil {
		t.Fatal("container adapter is nil")
	}
	if adapters.Session == nil {
		t.Fatal("session adapter is nil")
	}
}

func TestAdaptersは未対応Providerをエラーにする(t *testing.T) {
	_, err := NewAdapters(domain.ProviderConfig{
		Source:    "svn",
		Container: "docker",
		Session:   "tmux",
	}, nil)

	if err == nil {
		t.Fatal("未対応providerなのにエラーが返らなかった")
	}
	if err.Error() != `unsupported providers.source "svn"` {
		t.Fatalf("error = %q, want %q", err.Error(), `unsupported providers.source "svn"`)
	}
}
```

- [ ] **Step 2: Run provider selection tests to verify they fail**

Run:

```bash
go test ./internal/adapter/provider -run 'TestAdapters'
```

Expected: FAIL with `undefined: NewAdapters`.

- [ ] **Step 3: Implement provider adapter selection**

Create `internal/adapter/provider/adapters.go`:

```go
package provider

import (
	"fmt"

	"github.com/shige1114/paradev/internal/adapter/container"
	"github.com/shige1114/paradev/internal/adapter/session"
	"github.com/shige1114/paradev/internal/adapter/source"
	"github.com/shige1114/paradev/internal/adapter/system"
	"github.com/shige1114/paradev/internal/domain"
	"github.com/shige1114/paradev/internal/usecase"
)

type Adapters struct {
	Source     usecase.SourcePort
	Containers usecase.ContainerPort
	Session    usecase.SessionPort
}

func NewAdapters(providers domain.ProviderConfig, runner system.Runner) (Adapters, error) {
	var adapters Adapters
	switch providers.Source {
	case "git":
		adapters.Source = source.GitSourceAdapter{Runner: runner}
	default:
		return Adapters{}, fmt.Errorf("unsupported providers.source %q", providers.Source)
	}
	switch providers.Container {
	case "docker":
		adapters.Containers = container.DockerCLIAdapter{Runner: runner}
	default:
		return Adapters{}, fmt.Errorf("unsupported providers.container %q", providers.Container)
	}
	switch providers.Session {
	case "tmux":
		adapters.Session = session.TmuxAdapter{Runner: runner}
	default:
		return Adapters{}, fmt.Errorf("unsupported providers.session %q", providers.Session)
	}
	return adapters, nil
}
```

- [ ] **Step 4: Run provider selection tests to verify they pass**

Run:

```bash
go test ./internal/adapter/provider -run 'TestAdapters'
```

Expected: PASS.

- [ ] **Step 5: Wire provider selection in `Run`**

In `internal/app/cli.go`, remove direct adapter variables:

```go
	sourceAdapter := source.GitSourceAdapter{Runner: runner}
	containerAdapter := container.DockerCLIAdapter{Runner: runner}
	sessionAdapter := session.TmuxAdapter{Runner: runner}
```

In the `CommandCreate` case, load config once and select adapters:

```go
	case CommandCreate:
		cfg, err := configAdapter.Load(ctx)
		if err != nil {
			return err
		}
		adapters, err := provider.NewAdapters(cfg.Providers, runner)
		if err != nil {
			return err
		}
		uc := usecase.CreateCellUseCase{
			Config:     staticConfig{cfg: cfg},
			State:      stateAdapter,
			Source:     adapters.Source,
			Containers: adapters.Containers,
			Session:    adapters.Session,
			IDs:        id.RandomGenerator{},
		}
		_, err = uc.Execute(ctx, usecase.CreateCellInput{Issue: cmd.Issue, Template: cmd.Template})
		return err
```

In the `CommandRemove` case, load config and select adapters:

```go
	case CommandRemove:
		cfg, err := configAdapter.Load(ctx)
		if err != nil {
			return err
		}
		adapters, err := provider.NewAdapters(cfg.Providers, runner)
		if err != nil {
			return err
		}
		uc := usecase.RemoveCellUseCase{
			State:      stateAdapter,
			Source:     adapters.Source,
			Containers: adapters.Containers,
			Session:    adapters.Session,
		}
		return uc.Execute(ctx, usecase.RemoveCellInput{Cell: cmd.Cell})
```

Create a small config port implementation in `internal/app/providers.go`:

```go
type staticConfig struct {
	cfg domain.Config
}

func (s staticConfig) Load(ctx context.Context) (domain.Config, error) {
	return s.cfg, nil
}
```

Update `internal/app/providers.go` imports to include `context`.

- [ ] **Step 6: Run app tests to verify they pass**

Run:

```bash
go test ./internal/app
```

Expected: PASS.

- [ ] **Step 7: Commit Task 5**

```bash
git add internal/app/cli.go internal/app/providers.go internal/adapter/provider/adapters.go internal/adapter/provider/adapters_test.go
git commit -m "Select adapters from provider config"
```

## Task 6: Command Behavior Regression Tests

**Files:**
- Modify: `internal/app/cli_test.go`

- [ ] **Step 1: Add tests for `ls` independence and create provider validation**

Add these tests to `internal/app/cli_test.go` before `Test未対応コマンドはエラーにする`:

```go
func TestRunはLsでPdevYmlがなくても成功する(t *testing.T) {
	dir := t.TempDir()

	output, err := captureStdout(func() error {
		return Run(context.Background(), []string{"ls"}, dir)
	})

	if err != nil {
		t.Fatalf("Runでエラーが返った: %v", err)
	}
	if output != "NAME\tTEMPLATE\n" {
		t.Fatalf("output = %q, want %q", output, "NAME\tTEMPLATE\n")
	}
}

func TestRunはCreateでProvidersがない設定をエラーにする(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".pdev.yml")
	content := []byte(`project:
  name: myapp
templates:
  default:
    repository:
      branchPrefix: feat/
      base: main
    containers:
      services: {}
    session:
      windows: []
`)
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		t.Fatalf("設定を書けなかった: %v", err)
	}

	err := Run(context.Background(), []string{"create", "123", "--template", "default"}, dir)

	if err == nil {
		t.Fatal("providersがないのにエラーが返らなかった")
	}
	if err.Error() != "providers.source is required" {
		t.Fatalf("error = %q, want %q", err.Error(), "providers.source is required")
	}
}
```

- [ ] **Step 2: Run app regression tests**

Run:

```bash
go test ./internal/app -run 'TestRunはLsでPdevYmlがなくても成功する|TestRunはCreateでProvidersがない設定をエラーにする'
```

Expected: PASS if previous tasks are complete.

- [ ] **Step 3: Commit Task 6**

```bash
git add internal/app/cli_test.go
git commit -m "Cover provider config command behavior"
```

## Task 7: Full Verification

**Files:**
- No source edits expected.

- [ ] **Step 1: Run full tests**

Run:

```bash
go test ./...
```

Expected: all packages pass.

- [ ] **Step 2: Verify init output includes providers**

Run in a temporary copy or after moving the existing generated `.pdev.yml` aside:

```bash
go run ./cmd/pdev init
```

Expected `.pdev.yml` includes:

```yaml
providers:
    source: git
    container: docker
    session: tmux
```

- [ ] **Step 3: Verify `pdev ls` still does not require config**

Run:

```bash
go run ./cmd/pdev ls
```

Expected: exits 0 and prints at least:

```text
NAME	TEMPLATE
```

- [ ] **Step 4: Commit verification docs only if necessary**

No commit is needed if verification does not change files. If the plan is updated based on verification, commit only the plan update:

```bash
git add docs/superpowers/plans/2026-05-24-provider-config.md
git commit -m "Document provider config verification"
```
