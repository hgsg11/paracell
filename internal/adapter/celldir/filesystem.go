package celldir

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// FilesystemAdapter manages the parts of a Cell directory that are not
// worktrees: the files copied from the workspace and the directory itself.
// Paths are relative to Root, the workspace.
type FilesystemAdapter struct {
	Root string
}

func NewFilesystemAdapter(root string) FilesystemAdapter { return FilesystemAdapter{Root: root} }

// CopyWorkspaceFiles copies each workspace path to the same path inside the
// Cell directory. Files are copied, not linked, and replace what is already at
// the destination, including files the worktree tracks. A missing workspace
// path is an error.
func (a FilesystemAdapter) CopyWorkspaceFiles(ctx context.Context, cellDirectory string, paths []string) error {
	cell, err := a.cellPath(cellDirectory)
	if err != nil {
		return err
	}
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return err
		}
		// A listed path that is a symlink is copied as what it points to, so
		// the Cell never shares the file with the workspace.
		source, err := filepath.EvalSymlinks(filepath.Join(a.Root, path))
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("workspace.copy path %q does not exist in the workspace", path)
			}
			return fmt.Errorf("workspace.copy path %q: %w", path, err)
		}
		if err := copyTree(source, filepath.Join(cell, path)); err != nil {
			return fmt.Errorf("copy workspace path %q: %w", path, err)
		}
	}
	return nil
}

// RemoveCellDirectory removes the Cell directory with everything left in it.
// A missing directory is not an error.
func (a FilesystemAdapter) RemoveCellDirectory(_ context.Context, cellDirectory string) error {
	cell, err := a.cellPath(cellDirectory)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(cell); err != nil {
		return fmt.Errorf("remove cell directory %q: %w", cellDirectory, err)
	}
	return nil
}

// cellPath resolves the Cell directory and refuses anything that is not
// strictly inside the workspace, so a bad value never removes the workspace.
func (a FilesystemAdapter) cellPath(cellDirectory string) (string, error) {
	clean := filepath.Clean(cellDirectory)
	if cellDirectory == "" || filepath.IsAbs(clean) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("cell directory %q must be inside the workspace", cellDirectory)
	}
	return filepath.Join(a.Root, clean), nil
}

// copyTree copies source to destination. Directories are merged, while files
// and symlinks replace whatever is at their destination. Symlinks below source
// are copied as symlinks.
func copyTree(source string, destination string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, rel)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if existing, err := os.Lstat(target); err == nil && !existing.IsDir() {
				if err := os.Remove(target); err != nil {
					return err
				}
			}
			return os.MkdirAll(target, info.Mode().Perm()|0o700)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.RemoveAll(target); err != nil {
			return err
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%q is not a regular file, directory or symlink", path)
		}
		return copyFile(path, target, info.Mode().Perm())
	})
}

func copyFile(source string, destination string, mode fs.FileMode) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
