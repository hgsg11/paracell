package domain

import (
	"context"
)

type SourceCreationPort interface {
	CreateSource(ctx context.Context, repository string, worktree string, base string, branch string) error
}

func CreateSourcesService(ctx context.Context, templates []SourceTemplate, issue string, sourcePort SourceCreationPort) error {
	for _, template := range templates {
		source, err := NewSource(template.Path, template.Base, template.Prefix+issue)
		if err != nil {
			return err
		}
		worktree := cellWorktreePath(issue, source.Path)
		if err := sourcePort.CreateSource(ctx, source.Path, worktree, source.Base, source.Branch); err != nil {
			return err
		}
	}
	return nil
}
