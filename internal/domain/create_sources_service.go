package domain

import (
	"context"
)

type SourceCreationPort interface {
	CreateSource(ctx context.Context, repository string, worktree string, base string, branch string) error
}

func CreateSourcesService(ctx context.Context, templates []SourceTemplate, issue string, sourcePort SourceCreationPort) ([]Source, error) {
	sources := make([]Source, 0, len(templates))
	for _, template := range templates {
		source, err := NewSource(template.Path, template.WorktreePath(issue), template.Base, template.BranchName(issue))
		if err != nil {
			return nil, err
		}
		sources = append(sources, source)
	}
	for _, source := range sources {
		if err := sourcePort.CreateSource(ctx, source.Path, source.Worktree, source.Base, source.Branch); err != nil {
			return sources, err
		}
	}
	return sources, nil
}
