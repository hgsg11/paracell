package source

import (
	"context"
	"errors"
	"fmt"
	"os"
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

func (a GitSourceAdapter) CreateSource(ctx context.Context, source domain.SourceResource) (bool, error) {
	existed, err := a.branchExists(ctx, source)
	if err != nil {
		return false, err
	}
	args := append(a.gitArgs(source), "worktree", "add", a.worktreePath(source), "-b", source.Branch)
	if source.Base != "" && source.Base != "current" {
		args = append(args, source.Base)
	}
	if err := a.Runner.Run(ctx, "git", args...); err != nil {
		return !existed, err
	}
	return !existed, nil
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

func (a GitSourceAdapter) branchExists(ctx context.Context, source domain.SourceResource) (bool, error) {
	args := append(a.gitArgs(source), "show-ref", "--verify", "--quiet", "refs/heads/"+source.Branch)
	err := a.Runner.Run(ctx, "git", args...)
	if err == nil {
		return true, nil
	}
	var exitCoder interface{ ExitCode() int }
	if errors.As(err, &exitCoder) && exitCoder.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("check git branch %q: %w", source.Branch, err)
}

func (a GitSourceAdapter) ResumeSource(ctx context.Context, source domain.SourceResource) error {
	path := a.worktreePath(source)
	if _, err := os.Stat(filepath.Join(path, ".git")); err == nil {
		branch, branchErr := a.Runner.Output(ctx, "git", "-C", path, "branch", "--show-current")
		if branchErr != nil {
			return fmt.Errorf("inspect existing worktree %q: %w", source.WorktreePath, branchErr)
		}
		if strings.TrimSpace(branch) != source.Branch {
			return fmt.Errorf("worktree %q uses branch %q, want %q", source.WorktreePath, strings.TrimSpace(branch), source.Branch)
		}
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect worktree %q: %w", source.WorktreePath, err)
	}
	exists, err := a.branchExists(ctx, source)
	if err != nil {
		return err
	}
	if exists {
		args := append(a.gitArgs(source), "worktree", "add", path, source.Branch)
		return a.Runner.Run(ctx, "git", args...)
	}
	args := append(a.gitArgs(source), "worktree", "add", path, "-b", source.Branch)
	if source.Base != "" && source.Base != "current" {
		args = append(args, source.Base)
	}
	return a.Runner.Run(ctx, "git", args...)
}

func (a GitSourceAdapter) CleanSource(ctx context.Context, source domain.SourceResource) error {
	args := append(a.gitArgs(source), "worktree", "remove", "--force", a.worktreePath(source))
	err := a.Runner.Run(ctx, "git", args...)
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "is not a working tree") {
		return fmt.Errorf("%w: %v", domain.ErrNotFound, err)
	}
	return err
}
