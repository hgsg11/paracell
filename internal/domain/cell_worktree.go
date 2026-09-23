package domain

import "path/filepath"

type CellWorktree struct {
	Path string
}

func NewCellWorktree(cellName CellName, source Source) CellWorktree {
	path := filepath.Join(".paracell", "cells", cellName.Value, "source")
	if source.Path != "." {
		path = filepath.Join(path, source.Path)
	}
	return CellWorktree{Path: path}
}
