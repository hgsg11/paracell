package domain

import "path/filepath"

func cellWorktreePath(issue string, sourcePath string) string {
	worktree := filepath.Join(".paracell", "cells", NewCellName(issue).Value, "source")
	if sourcePath != "." {
		worktree = filepath.Join(worktree, sourcePath)
	}
	return worktree
}
