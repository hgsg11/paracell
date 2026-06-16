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
	if cell.BranchMode == domain.RepositoryBranchModeRequire {
		return a.Runner.Run(ctx, "git", "worktree", "add", cell.Source.Path, cell.Branch)
	}
	if cell.BranchMode == domain.RepositoryBranchModeReuse && a.branchExists(ctx, cell.Branch) {
		return a.Runner.Run(ctx, "git", "worktree", "add", cell.Source.Path, cell.Branch)
	}
	args := []string{"worktree", "add", cell.Source.Path, "-b", cell.Branch}
	if cell.Base != "" && cell.Base != "current" {
		args = append(args, cell.Base)
	}
	return a.Runner.Run(ctx, "git", args...)
}

func (a GitSourceAdapter) branchExists(ctx context.Context, branch string) bool {
	return a.Runner.Run(ctx, "git", "show-ref", "--verify", "--quiet", "refs/heads/"+branch) == nil
}

func (a GitSourceAdapter) CleanSource(ctx context.Context, cell domain.Cell) error {
	return a.Runner.Run(ctx, "git", "worktree", "remove", "--force", cell.Source.Path)
}
