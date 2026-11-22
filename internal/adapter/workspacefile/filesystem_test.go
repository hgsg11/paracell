package workspacefile

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hgsg11/paracell/internal/domain"
)

func TestExistingPathsはファイルだけをあるとしディレクトリと無いパスを除く(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"file.go", "dir/nested.go"} {
		if err := os.WriteFile(filepath.Join(root, file), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("missing-target", filepath.Join(root, "link.go")); err != nil {
		t.Fatal(err)
	}
	var paths []domain.WorkspacePath
	for _, value := range []string{"file.go", "dir/nested.go", "dir", "missing.go", "file.go/child.go", "link.go"} {
		path, err := domain.NewWorkspacePath(value)
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}
	existing, err := NewFilesystemAdapter(root).ExistingPaths(context.Background(), paths)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"file.go": true, "dir/nested.go": true, "dir": false, "missing.go": false, "file.go/child.go": false, "link.go": true}
	for _, path := range paths {
		if existing.Contains(path) != want[path.Value] {
			t.Fatalf("Contains(%q) = %v, want %v", path.Value, existing.Contains(path), want[path.Value])
		}
	}
}
