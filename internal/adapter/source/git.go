package source

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/hgsg11/paracell/internal/adapter/system"
	"github.com/hgsg11/paracell/internal/domain"
)

type GitSourceAdapter struct {
	Runner system.Runner
	Root   string
}

func NewGitSourceAdapter(runner system.Runner, root string) GitSourceAdapter {
	return GitSourceAdapter{Runner: runner, Root: root}
}

func (a GitSourceAdapter) CreateSource(ctx context.Context, repository string, worktree string, base string, branch string) error {
	args := append(a.gitArgs(repository), "worktree", "add", a.worktreePath(worktree), "-b", branch)
	if base != "" && base != "current" {
		args = append(args, base)
	}
	return a.Runner.Run(ctx, "git", args...)
}

func (a GitSourceAdapter) worktreePath(worktree string) string {
	if filepath.IsAbs(worktree) || a.Root == "" {
		return worktree
	}
	return filepath.Join(a.Root, worktree)
}

func (a GitSourceAdapter) gitArgs(repository string) []string {
	if repository == "" {
		repository = "."
	}
	if !filepath.IsAbs(repository) && a.Root != "" {
		repository = filepath.Join(a.Root, repository)
	}
	return []string{"-C", repository}
}

func (a GitSourceAdapter) CleanSource(ctx context.Context, repository string, worktree string) error {
	args := append(a.gitArgs(repository), "worktree", "remove", "--force", a.worktreePath(worktree))
	err := a.Runner.Run(ctx, "git", args...)
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "is not a working tree") {
		return fmt.Errorf("%w: %v", domain.ErrNotFound, err)
	}
	return err
}
