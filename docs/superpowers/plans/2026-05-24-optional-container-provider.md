# Optional Container Provider Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow `providers.container` to be omitted or empty so `pdev` can create and remove environments without Docker/container operations.

**Architecture:** Keep usecases unchanged by introducing a no-op `ContainerPort` adapter. The YAML config adapter validates source/session as required but treats container as optional. Provider selection maps empty container provider to the no-op adapter and `docker` to the existing Docker adapter.

**Tech Stack:** Go 1.26, standard library, `gopkg.in/yaml.v3`, existing Clean Architecture package layout.

---

## File Structure

- Modify `internal/adapter/config/yaml_config.go`: allow empty `providers.container` while rejecting unsupported non-empty values.
- Modify `internal/adapter/config/yaml_config_test.go`: cover missing, empty, and unsupported container provider values.
- Create `internal/adapter/container/noop.go`: no-op container adapter implementing `usecase.ContainerPort`.
- Create `internal/adapter/container/noop_test.go`: prove no-op create/remove return nil.
- Modify `internal/adapter/provider/adapters.go`: map empty container provider to no-op adapter.
- Modify `internal/adapter/provider/adapters_test.go`: cover no-op provider selection and unsupported non-empty container provider.
- Modify `internal/app/cli_test.go`: regression tests that create/remove with no container provider do not run Docker commands.

## Task 1: YAML Validation Allows Missing Container

**Files:**
- Modify: `internal/adapter/config/yaml_config.go`
- Modify: `internal/adapter/config/yaml_config_test.go`

- [ ] **Step 1: Write failing YAML tests for optional container**

Add these tests to `internal/adapter/config/yaml_config_test.go`:

```go
func TestYAML設定はContainerProviderがない場合も読み込める(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".pdev.yml")
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
	cfg, err := loader.Load(context.Background())

	if err != nil {
		t.Fatalf("設定読み込みでエラーが返った: %v", err)
	}
	if cfg.Providers.Container != "" {
		t.Fatalf("providers.container = %q, want empty", cfg.Providers.Container)
	}
}

func TestYAML設定はContainerProviderが空文字でも読み込める(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".pdev.yml")
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
	cfg, err := loader.Load(context.Background())

	if err != nil {
		t.Fatalf("設定読み込みでエラーが返った: %v", err)
	}
	if cfg.Providers.Container != "" {
		t.Fatalf("providers.container = %q, want empty", cfg.Providers.Container)
	}
}

func TestYAML設定は未対応ContainerProviderの場合に失敗する(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".pdev.yml")
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
	_, err := loader.Load(context.Background())

	if err == nil {
		t.Fatal("未対応container providerなのにエラーが返らなかった")
	}
	if err.Error() != `unsupported providers.container "podman"` {
		t.Fatalf("error = %q, want %q", err.Error(), `unsupported providers.container "podman"`)
	}
}
```

- [ ] **Step 2: Run YAML tests to verify they fail**

Run:

```bash
go test ./internal/adapter/config -run 'TestYAML設定はContainerProvider|TestYAML設定は未対応ContainerProvider'
```

Expected: FAIL because empty container provider currently returns `providers.container is required`.

- [ ] **Step 3: Relax container validation**

Modify `validateProviders` in `internal/adapter/config/yaml_config.go`:

```go
func validateProviders(providers domain.ProviderConfig) error {
	if providers.Source == "" {
		return errors.New("providers.source is required")
	}
	if providers.Source != "git" {
		return fmt.Errorf("unsupported providers.source %q", providers.Source)
	}
	if providers.Container != "" && providers.Container != "docker" {
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

- [ ] **Step 4: Run YAML tests to verify they pass**

Run:

```bash
go test ./internal/adapter/config -run 'TestYAML設定はContainerProvider|TestYAML設定は未対応ContainerProvider'
```

Expected: PASS.

- [ ] **Step 5: Commit Task 1**

```bash
git add internal/adapter/config/yaml_config.go internal/adapter/config/yaml_config_test.go
git commit -m "Allow optional container provider in config"
```

## Task 2: No-op Container Adapter

**Files:**
- Create: `internal/adapter/container/noop.go`
- Create: `internal/adapter/container/noop_test.go`

- [ ] **Step 1: Write failing no-op adapter tests**

Create `internal/adapter/container/noop_test.go`:

```go
package container

import (
	"context"
	"testing"

	"github.com/shige1114/paradev/internal/domain"
)

func TestNoopAdapterはCreateContainersで何もしない(t *testing.T) {
	err := NoopAdapter{}.CreateContainers(context.Background(), domain.Cell{}, domain.Template{})

	if err != nil {
		t.Fatalf("CreateContainers error = %v, want nil", err)
	}
}

func TestNoopAdapterはRemoveContainersで何もしない(t *testing.T) {
	err := NoopAdapter{}.RemoveContainers(context.Background(), domain.Cell{})

	if err != nil {
		t.Fatalf("RemoveContainers error = %v, want nil", err)
	}
}
```

- [ ] **Step 2: Run no-op adapter tests to verify they fail**

Run:

```bash
go test ./internal/adapter/container -run 'TestNoopAdapter'
```

Expected: FAIL with `undefined: NoopAdapter`.

- [ ] **Step 3: Implement no-op adapter**

Create `internal/adapter/container/noop.go`:

```go
package container

import (
	"context"

	"github.com/shige1114/paradev/internal/domain"
)

type NoopAdapter struct{}

func (a NoopAdapter) CreateContainers(ctx context.Context, cell domain.Cell, template domain.Template) error {
	_ = ctx
	_ = cell
	_ = template
	return nil
}

func (a NoopAdapter) RemoveContainers(ctx context.Context, cell domain.Cell) error {
	_ = ctx
	_ = cell
	return nil
}
```

- [ ] **Step 4: Run no-op adapter tests to verify they pass**

Run:

```bash
go test ./internal/adapter/container -run 'TestNoopAdapter'
```

Expected: PASS.

- [ ] **Step 5: Commit Task 2**

```bash
git add internal/adapter/container/noop.go internal/adapter/container/noop_test.go
git commit -m "Add no-op container adapter"
```

## Task 3: Provider Selection Uses No-op Container

**Files:**
- Modify: `internal/adapter/provider/adapters.go`
- Modify: `internal/adapter/provider/adapters_test.go`

- [ ] **Step 1: Write failing provider selection tests**

Add these tests to `internal/adapter/provider/adapters_test.go`:

```go
func TestAdaptersはContainerProviderが空ならNoopContainerを選択する(t *testing.T) {
	adapters, err := NewAdapters(domain.ProviderConfig{
		Source:  "git",
		Session: "tmux",
	}, nil)

	if err != nil {
		t.Fatalf("provider adapter選択でエラーが返った: %v", err)
	}
	if adapters.Containers == nil {
		t.Fatal("container adapter is nil")
	}
}

func TestAdaptersは未対応ContainerProviderをエラーにする(t *testing.T) {
	_, err := NewAdapters(domain.ProviderConfig{
		Source:    "git",
		Container: "podman",
		Session:   "tmux",
	}, nil)

	if err == nil {
		t.Fatal("未対応container providerなのにエラーが返らなかった")
	}
	if err.Error() != `unsupported providers.container "podman"` {
		t.Fatalf("error = %q, want %q", err.Error(), `unsupported providers.container "podman"`)
	}
}
```

- [ ] **Step 2: Run provider selection tests to verify they fail**

Run:

```bash
go test ./internal/adapter/provider -run 'TestAdaptersはContainerProvider|TestAdaptersは未対応ContainerProvider'
```

Expected: FAIL because empty container provider is unsupported.

- [ ] **Step 3: Map empty container provider to no-op adapter**

Modify the container switch in `internal/adapter/provider/adapters.go`:

```go
	switch providers.Container {
	case "":
		adapters.Containers = container.NoopAdapter{}
	case "docker":
		adapters.Containers = container.DockerCLIAdapter{Runner: runner}
	default:
		return Adapters{}, fmt.Errorf("unsupported providers.container %q", providers.Container)
	}
```

- [ ] **Step 4: Run provider selection tests to verify they pass**

Run:

```bash
go test ./internal/adapter/provider -run 'TestAdaptersはContainerProvider|TestAdaptersは未対応ContainerProvider'
```

Expected: PASS.

- [ ] **Step 5: Commit Task 3**

```bash
git add internal/adapter/provider/adapters.go internal/adapter/provider/adapters_test.go
git commit -m "Use no-op adapter for empty container provider"
```

## Task 4: App Behavior Without Container Provider

**Files:**
- Modify: `internal/app/cli_test.go`

- [ ] **Step 1: Add command behavior regression tests**

Add these tests to `internal/app/cli_test.go` before `Test未対応コマンドはエラーにする`:

```go
func TestRunはCreateでContainerProviderがなくてもDockerを実行しない(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".pdev.yml")
	content := []byte(`project:
  name: myapp
providers:
  source: git
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
`)
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		t.Fatalf("設定を書けなかった: %v", err)
	}

	err := Run(context.Background(), []string{"create", "123", "--template", "default"}, dir)

	if err == nil {
		t.Fatal("git/tmuxが実行できない環境なのにエラーが返らなかった")
	}
	if err.Error() == `exec: "docker": executable file not found in $PATH` {
		t.Fatalf("container provider未設定なのにdockerが実行された: %v", err)
	}
}

func TestRunはRemoveでContainerProviderがなくてもDockerを実行しない(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".pdev.yml")
	content := []byte(`project:
  name: myapp
providers:
  source: git
  session: tmux
templates: {}
`)
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		t.Fatalf("設定を書けなかった: %v", err)
	}
	store := state.JSONCellStateAdapter{Path: filepath.Join(dir, ".pdev", "state.json")}
	if err := store.SaveCells(context.Background(), []domain.Cell{
		{
			ID:    "cell-1",
			Issue: "123",
			Name:  "123",
			Source: domain.Source{
				Path: filepath.Join(dir, "missing-source"),
			},
			Containers: domain.Containers{
				Network: "pdev-myapp-123",
				Services: map[string]domain.CellContainer{
					"web": {ContainerName: "pdev-myapp-123-web"},
				},
			},
			Session: domain.Session{Name: "pdev-myapp-123"},
		},
	}); err != nil {
		t.Fatalf("state保存でエラーが返った: %v", err)
	}

	err := Run(context.Background(), []string{"remove", "123"}, dir)

	if err == nil {
		t.Fatal("tmux/gitが実行できない環境なのにエラーが返らなかった")
	}
	if err.Error() == `exec: "docker": executable file not found in $PATH` {
		t.Fatalf("container provider未設定なのにdockerが実行された: %v", err)
	}
}
```

These tests are intentionally coarse: the current app wiring uses real git and
tmux adapters, so the command may fail at git or tmux in the test environment.
The behavior under test is that Docker is not invoked when the container
provider is omitted.

- [ ] **Step 2: Run app behavior tests**

Run:

```bash
go test ./internal/app -run 'TestRunはCreateでContainerProviderがなくてもDockerを実行しない|TestRunはRemoveでContainerProviderがなくてもDockerを実行しない'
```

Expected: PASS if previous tasks are complete.

- [ ] **Step 3: Commit Task 4**

```bash
git add internal/app/cli_test.go
git commit -m "Cover commands without container provider"
```

## Task 5: Full Verification

**Files:**
- No source edits expected.

- [ ] **Step 1: Run full tests**

Run:

```bash
go test ./...
```

Expected: all packages pass.

- [ ] **Step 2: Verify `pdev ls` still works**

Run:

```bash
go run ./cmd/pdev ls
```

Expected: exits 0 and prints:

```text
NAME	TEMPLATE
```

- [ ] **Step 3: Verify init still emits Docker by default**

Run in a temporary copy or after moving the existing generated `.pdev.yml` aside:

```bash
go run ./cmd/pdev init
```

Expected `.pdev.yml` still includes:

```yaml
providers:
    source: git
    container: docker
    session: tmux
```

- [ ] **Step 4: Commit verification docs only if necessary**

No commit is needed if verification does not change files. If the plan is updated based on verification, commit only the plan update:

```bash
git add docs/superpowers/plans/2026-05-24-optional-container-provider.md
git commit -m "Document optional container provider verification"
```
