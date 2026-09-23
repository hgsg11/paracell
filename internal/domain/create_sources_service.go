package domain

import "context"

type SourceCreationPort interface {
	CreateSource(ctx context.Context, template SourceTemplate, worktree string, branch string) error
}

func CreateSourcesService(ctx context.Context, cell Cell, templates []SourceTemplate, sourcePort SourceCreationPort) error {
	for i, template := range templates {
		source := cell.Sources.Items[i]
		if err := sourcePort.CreateSource(ctx, template, cell.SourceWorktreePath(source), source.Branch); err != nil {
			return err
		}
	}
	return nil
}
