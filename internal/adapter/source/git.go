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
	args := []string{"worktree", "add", cell.Source.Path, "-b", cell.Branch}
	if cell.Base != "" && cell.Base != "current" {
		args = append(args, cell.Base)
	}
	return a.Runner.Run(ctx, "git", args...)
}

func (a GitSourceAdapter) CleanSource(ctx context.Context, cell domain.Cell) error {
	return a.Runner.Run(ctx, "git", "worktree", "remove", "--force", cell.Source.Path)
}
