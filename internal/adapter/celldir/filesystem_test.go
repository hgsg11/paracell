package celldir

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func TestCopyWorkspaceFilesはワークスペースの版で上書きする(t *testing.T) {
	root := t.TempDir()
	cell := filepath.Join(root, ".paracell/cells/42")
	writeFile(t, filepath.Join(root, ".claude/settings.json"), "workspace")
	writeFile(t, filepath.Join(root, ".claude/agents/a.md"), "agent")
	writeFile(t, filepath.Join(root, ".env.local"), "SECRET=1")
	// The worktree already holds a tracked version and a file the workspace lacks.
	writeFile(t, filepath.Join(cell, ".claude/settings.json"), "tracked")
	writeFile(t, filepath.Join(cell, ".claude/keep.md"), "keep")

	if err := NewFilesystemAdapter(root).CopyWorkspaceFiles(context.Background(), ".paracell/cells/42", []string{".claude", ".env.local"}); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{
		".claude/settings.json": "workspace",
		".claude/agents/a.md":   "agent",
		".claude/keep.md":       "keep",
		".env.local":            "SECRET=1",
	} {
		if got := readFile(t, filepath.Join(cell, path)); got != want {
			t.Errorf("%s = %q, want %q", path, got, want)
		}
	}
	info, err := os.Lstat(filepath.Join(cell, ".claude"))
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		t.Fatalf(".claude is not a copied directory: %v %v", info, err)
	}
}

func TestCopyWorkspaceFilesはSymlinkの指す実体をコピーする(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "shared/env"), "shared")
	if err := os.Symlink("shared/env", filepath.Join(root, ".env.local")); err != nil {
		t.Fatal(err)
	}
	if err := NewFilesystemAdapter(root).CopyWorkspaceFiles(context.Background(), ".paracell/cells/42", []string{".env.local"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".paracell/cells/42/.env.local")
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("copy is a symlink: %v %v", info, err)
	}
	if got := readFile(t, path); got != "shared" {
		t.Fatalf("content = %q", got)
	}
}

func TestCopyWorkspaceFilesは存在しないパスで失敗する(t *testing.T) {
	root := t.TempDir()
	err := NewFilesystemAdapter(root).CopyWorkspaceFiles(context.Background(), ".paracell/cells/42", []string{"missing"})
	if err == nil || !strings.Contains(err.Error(), `"missing" does not exist`) {
		t.Fatalf("err = %v", err)
	}
}

func TestRemoveCellDirectoryはCellのディレクトリごと消す(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".paracell/cells/42/.claude/settings.json"), "copied")
	writeFile(t, filepath.Join(root, ".paracell/cells/43/keep"), "other")
	adapter := NewFilesystemAdapter(root)
	if err := adapter.RemoveCellDirectory(context.Background(), ".paracell/cells/42"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".paracell/cells/42")); !os.IsNotExist(err) {
		t.Fatalf("cell directory remains: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".paracell/cells/43/keep")); err != nil {
		t.Fatalf("other cell was removed: %v", err)
	}
	if err := adapter.RemoveCellDirectory(context.Background(), ".paracell/cells/42"); err != nil {
		t.Fatalf("missing directory: %v", err)
	}
}

func TestRemoveCellDirectoryはワークスペースの外やルートを消さない(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{"", ".", "..", "../other", "/tmp"} {
		if err := NewFilesystemAdapter(root).RemoveCellDirectory(context.Background(), path); err == nil {
			t.Fatalf("RemoveCellDirectory(%q) was accepted", path)
		}
	}
	if _, err := os.Stat(root); err != nil {
		t.Fatal(err)
	}
}
