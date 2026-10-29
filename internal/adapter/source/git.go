package source

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/hgsg11/paracell/internal/adapter/system"
	"github.com/hgsg11/paracell/internal/domain"
)

const originRemote = "origin"

type GitSourceAdapter struct {
	Runner system.Runner
	Root   string
}

func NewGitSourceAdapter(runner system.Runner, root string) GitSourceAdapter {
	return GitSourceAdapter{Runner: runner, Root: root}
}

func (a GitSourceAdapter) CreateSource(ctx context.Context, source domain.SourceResource) error {
	if err := a.fetch(ctx, source); err != nil {
		return err
	}
	return a.addWorktree(ctx, source)
}

func (a GitSourceAdapter) fetch(ctx context.Context, source domain.SourceResource) error {
	ok, err := a.hasOrigin(ctx, source)
	if err != nil || !ok {
		return err
	}
	return a.Runner.Run(ctx, "git", append(a.gitArgs(source), "fetch", originRemote)...)
}

func (a GitSourceAdapter) hasOrigin(ctx context.Context, source domain.SourceResource) (bool, error) {
	output, err := a.Runner.Output(ctx, "git", append(a.gitArgs(source), "remote")...)
	if err != nil {
		return false, err
	}
	for _, remote := range strings.Fields(output) {
		if remote == originRemote {
			return true, nil
		}
	}
	return false, nil
}

// hasRef reports whether ref exists. show-ref exits 1 only when the ref is missing.
func (a GitSourceAdapter) hasRef(ctx context.Context, source domain.SourceResource, ref string) (bool, error) {
	err := a.Runner.Run(ctx, "git", append(a.gitArgs(source), "show-ref", "--verify", "--quiet", ref)...)
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, err
}

func (a GitSourceAdapter) addWorktree(ctx context.Context, source domain.SourceResource) error {
	args := append(a.gitArgs(source), "worktree", "add")
	local, err := a.hasRef(ctx, source, "refs/heads/"+source.Branch)
	if err != nil {
		return err
	}
	if local {
		return a.Runner.Run(ctx, "git", append(args, a.worktreePath(source), source.Branch)...)
	}
	remote, err := a.hasRef(ctx, source, "refs/remotes/"+originRemote+"/"+source.Branch)
	if err != nil {
		return err
	}
	if remote {
		return a.Runner.Run(ctx, "git", append(args, "--track", "-b", source.Branch, a.worktreePath(source), originRemote+"/"+source.Branch)...)
	}
	args = append(args, "-b", source.Branch, a.worktreePath(source))
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
