package domain

import "context"

type SourceCreationPort interface {
	CreateSource(ctx context.Context, repository string, worktree string, base string, branch string) error
}

func CreateSourcesService(ctx context.Context, cell *Cell, port SourceCreationPort) error {
	for _, source := range cell.Sources.Items {
		if err := port.CreateSource(ctx, source.Path, source.Worktree, source.Base, source.Branch); err != nil {
			return err
		}
	}
	return nil
}
