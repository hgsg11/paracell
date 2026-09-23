package domain

import (
	"context"
)

type SourceCreationPort interface {
	CreateSource(ctx context.Context, source Source, worktree string) error
}

func CreateSourcesService(ctx context.Context, templates []SourceTemplate, issue string, sourcePort SourceCreationPort) error {
	for _, template := range templates {
		source, err := NewSource(template.Path, template.Base, template.Prefix+issue)
		if err != nil {
			return err
		}
		worktree := cellWorktreePath(issue, source.Path)
		if err := sourcePort.CreateSource(ctx, source, worktree); err != nil {
			return err
		}
	}
	return nil
}
