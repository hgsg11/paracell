package domain

import (
	"context"
)

type SourceCreationPort interface {
	CreateSource(ctx context.Context, repository string, worktree string, base string, branch string) error
}

func CreateSourcesService(ctx context.Context, sources []Source, sourcePort SourceCreationPort) error {
	for _, source := range sources {
		if err := sourcePort.CreateSource(ctx, source.Path, source.Worktree, source.Base, source.Branch); err != nil {
			return err
		}
	}
	return nil
}
