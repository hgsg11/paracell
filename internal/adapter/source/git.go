package source

import (
	"context"

	"github.com/hgsg11/paracell/internal/adapter/system"
	"github.com/hgsg11/paracell/internal/domain"
)

type GitSourceAdapter struct {
	Runner system.Runner
}

func (a GitSourceAdapter) CreateSource(ctx context.Context, cell domain.Cell) error {
	return a.Runner.Run(ctx, "git", "worktree", "add", cell.Source.Path, "-b", cell.Branch)
}

func (a GitSourceAdapter) RemoveSource(ctx context.Context, cell domain.Cell) error {
	return a.Runner.Run(ctx, "git", "worktree", "remove", "--force", cell.Source.Path)
}
