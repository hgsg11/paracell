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

func (a GitSourceAdapter) CreateSource(ctx context.Context, source domain.SourceResource) error {
	args := append(a.gitArgs(source), "worktree", "add", a.worktreePath(source), "-b", source.Branch)
	if source.Base != "" && source.Base != "current" {
		args = append(args, source.Base)
	}
	return a.Runner.Run(ctx, "git", args...)
}

func (a GitSourceAdapter) worktreePath(source domain.SourceResource) string {
	if filepath.IsAbs(source.WorktreePath) || a.Root == "" {
		return source.WorktreePath
	}
	return filepath.Join(a.Root, source.WorktreePath)
}

func (a GitSourceAdapter) gitArgs(source domain.SourceResource) []string {
	repository := source.RepositoryPath
	if repository == "" {
		repository = "."
	}
	if !filepath.IsAbs(repository) && a.Root != "" {
		repository = filepath.Join(a.Root, repository)
	}
	return []string{"-C", repository}
}

func (a GitSourceAdapter) CleanSource(ctx context.Context, source domain.SourceResource) error {
	args := append(a.gitArgs(source), "worktree", "remove", "--force", a.worktreePath(source))
	err := a.Runner.Run(ctx, "git", args...)
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "is not a working tree") {
		return fmt.Errorf("%w: %v", domain.ErrNotFound, err)
	}
	return err
}
