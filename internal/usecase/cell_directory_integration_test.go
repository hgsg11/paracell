package usecase

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hgsg11/paracell/internal/adapter/celldir"
	"github.com/hgsg11/paracell/internal/adapter/source"
	"github.com/hgsg11/paracell/internal/domain"
)

// These tests run the source stage and clean with the real git and filesystem
// adapters, so they check the Cell directory on disk.

type execRunner struct{}

func (execRunner) Run(ctx context.Context, name string, args ...string) error {
	if output, err := exec.CommandContext(ctx, name, args...).CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, output)
	}
	return nil
}

func (execRunner) Output(ctx context.Context, name string, args ...string) (string, error) {
	output, err := exec.CommandContext(ctx, name, args...).Output()
	return string(output), err
}

type gitSourceFactory struct{ root string }

func (f gitSourceFactory) Source(domain.SourceDriverType) (SourcePort, error) {
	return source.NewGitSourceAdapter(execRunner{}, f.root), nil
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	args = append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.com"}, args...)
	if output, err := exec.Command("git", args...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}

func initRepository(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for path, content := range files {
		full := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "init", "-q")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-q", "--allow-empty", "-m", "init")
}

// newCellDirectoryPorts configures one template with the given target source
// paths and the real adapters rooted at root.
func newCellDirectoryPorts(t *testing.T, root string, sources map[string]string, workspaceCopy []string) (*fakePorts, ForkCellUseCase, CleanCellUseCase) {
	t.Helper()
	ports := newFakePorts()
	var targets []domain.TargetCellSpec
	for name, path := range sources {
		template, err := domain.NewSourceTemplate(path, "current")
		if err != nil {
			t.Fatal(err)
		}
		target, err := domain.NewTargetCellSpec(name, &template, nil)
		if err != nil {
			t.Fatal(err)
		}
		targets = append(targets, target)
	}
	commander, _ := domain.NewCommanderCellSpec("workspace", domain.NewWorkspaceTemplate(nil))
	template, err := domain.NewUnresolvedTemplate("feat", &commander, targets, nil)
	if err != nil {
		t.Fatal(err)
	}
	ports.config, err = domain.NewTemplates("myapp", []domain.Template{template}, domain.Tmux, domain.None, domain.Git, domain.NoNotification, nil, workspaceCopy)
	if err != nil {
		t.Fatal(err)
	}
	files := celldir.NewFilesystemAdapter(root)
	fork := newForkCellUseCase(ports)
	fork.SourceFactory, fork.CellDirectory = gitSourceFactory{root: root}, files
	clean := CleanCellUseCase{Cells: ports, SourceFactory: gitSourceFactory{root: root}, CellDirectory: files, ContainerFactory: ports, WorkspaceFactory: ports}
	return ports, fork, clean
}

func assertExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

func assertMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("%s exists: %v", path, err)
	}
}

func TestSourceステージはCellのディレクトリをワークスペースの写しにしCleanで全部消す(t *testing.T) {
	root := t.TempDir()
	initRepository(t, root, map[string]string{".claude/settings.json": "tracked", ".gitignore": ".paracell/\n"})
	// The workspace version differs from the tracked one and is not committed.
	if err := os.WriteFile(filepath.Join(root, ".claude/settings.json"), []byte("workspace"), 0o644); err != nil {
		t.Fatal(err)
	}
	ports, fork, clean := newCellDirectoryPorts(t, root, map[string]string{"repository": ".", "docs": "."}, []string{".claude/"})
	if _, err := fork.Execute(context.Background(), ForkCellInput{Issue: "42", Template: "feat"}); err != nil {
		t.Fatal(err)
	}
	cell := filepath.Join(root, ".paracell/cells/42")
	assertExists(t, filepath.Join(cell, ".git"))
	assertMissing(t, filepath.Join(cell, "repository"))
	assertMissing(t, filepath.Join(cell, "docs"))
	content, err := os.ReadFile(filepath.Join(cell, ".claude/settings.json"))
	if err != nil || string(content) != "workspace" {
		t.Fatalf("copied settings = %q, %v", content, err)
	}
	// repository and docs share ".", so only one worktree is added and no
	// symlink is made.
	worktrees, err := exec.Command("git", "-C", root, "worktree", "list", "--porcelain").Output()
	if err != nil {
		t.Fatal(err)
	}
	if count := strings.Count(string(worktrees), "worktree "); count != 2 {
		t.Fatalf("worktrees:\n%s", worktrees)
	}

	if err := ports.cells.Commanders[0].MarkDone(); err != nil {
		t.Fatal(err)
	}
	if err := clean.Execute(context.Background(), CleanCellInput{Cell: "42"}); err != nil {
		t.Fatal(err)
	}
	assertMissing(t, cell)
}

func TestSourceステージはSourcePathごとにCellの同じ場所へWorktreeを作る(t *testing.T) {
	root := t.TempDir()
	initRepository(t, filepath.Join(root, "frontend"), map[string]string{"package.json": "{}"})
	initRepository(t, filepath.Join(root, "backend"), map[string]string{"go.mod": "module backend"})
	ports, fork, clean := newCellDirectoryPorts(t, root, map[string]string{"frontend": "frontend", "backend": "backend"}, nil)
	if _, err := fork.Execute(context.Background(), ForkCellInput{Issue: "42", Template: "feat"}); err != nil {
		t.Fatal(err)
	}
	cell := filepath.Join(root, ".paracell/cells/42")
	assertExists(t, filepath.Join(cell, "frontend/package.json"))
	assertExists(t, filepath.Join(cell, "backend/go.mod"))

	if err := ports.cells.Commanders[0].MarkDone(); err != nil {
		t.Fatal(err)
	}
	if err := clean.Execute(context.Background(), CleanCellInput{Cell: "42"}); err != nil {
		t.Fatal(err)
	}
	assertMissing(t, cell)
}

func TestSourceステージはリポジトリのサブディレクトリや存在しないコピー元で失敗する(t *testing.T) {
	for name, tt := range map[string]struct {
		sources map[string]string
		copy    []string
	}{
		"subdirectory": {sources: map[string]string{"api": "services/api"}},
		"missing copy": {sources: map[string]string{"repository": "."}, copy: []string{".env.local"}},
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			initRepository(t, root, map[string]string{"services/api/main.go": "package main", ".gitignore": ".paracell/\n"})
			ports, fork, _ := newCellDirectoryPorts(t, root, tt.sources, tt.copy)
			if _, err := fork.Execute(context.Background(), ForkCellInput{Issue: "42", Template: "feat"}); err == nil {
				t.Fatal("fork succeeded")
			}
			group := ports.cells.Groups[0]
			if group.Creation.Status != domain.CreationFailed || group.Creation.FailedStage != domain.CreationStageSource {
				t.Fatalf("creation = %#v", group.Creation)
			}
		})
	}
}
