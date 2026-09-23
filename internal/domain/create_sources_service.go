package domain

import (
	"context"
	"path/filepath"
)

type SourceCreationPort interface {
	CreateSource(ctx context.Context, template SourceTemplate, worktree string, branch string) error
}

func CreateSourcesService(ctx context.Context, templates []SourceTemplate, issue string, sourcePort SourceCreationPort) error {
	worktreeRoot := filepath.Join(".paracell", "cells", NewCellName(issue).Value, "source")
	for _, template := range templates {
		source, err := NewSource(template.Path, template.Base, template.Prefix+issue)
		if err != nil {
			return err
		}
		worktree := worktreeRoot
		if source.Path != "." {
			worktree = filepath.Join(worktree, source.Path)
		}
		if err := sourcePort.CreateSource(ctx, template, worktree, source.Branch); err != nil {
			return err
		}
	}
	return nil
}
