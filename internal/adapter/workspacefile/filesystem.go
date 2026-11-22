package workspacefile

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"

	"github.com/hgsg11/paracell/internal/domain"
)

// FilesystemAdapter checks workspace paths on disk. Paths are relative to Root,
// the workspace.
type FilesystemAdapter struct {
	Root string
}

func NewFilesystemAdapter(root string) FilesystemAdapter { return FilesystemAdapter{Root: root} }

// ExistingPaths returns the paths that exist under Root. It uses Lstat, so a
// symlink counts as the file itself; a directory counts as missing.
func (a FilesystemAdapter) ExistingPaths(ctx context.Context, paths []domain.WorkspacePath) (domain.ExistingPaths, error) {
	var existing []domain.WorkspacePath
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return domain.ExistingPaths{}, err
		}
		info, err := os.Lstat(filepath.Join(a.Root, filepath.FromSlash(path.Value)))
		// A parent that is a file is reported as ENOTDIR: the path is missing too.
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
			continue
		}
		if err != nil {
			return domain.ExistingPaths{}, fmt.Errorf("check workspace path %q: %w", path.Value, err)
		}
		if info.IsDir() {
			continue
		}
		existing = append(existing, path)
	}
	return domain.NewExistingPaths(existing), nil
}
