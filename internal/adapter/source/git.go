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
	"github.com/hgsg11/paracell/internal/usecase"
)

type GitSourceAdapter struct {
	Runner system.Runner
	Root   string
}

func (a GitSourceAdapter) CreateSource(ctx context.Context, cell domain.Cell) (usecase.SourceCreation, error) {
	created := false
	for _, source := range cell.Sources {
		existed, err := a.branchExists(ctx, source)
		if err != nil {
			return usecase.SourceCreation{BranchCreated: created}, err
		}
		args := append(a.gitArgs(source), "worktree", "add", a.worktreePath(source), "-b", source.Branch)
		if source.Base != "" && source.Base != "current" {
			args = append(args, source.Base)
		}
		if err := a.Runner.Run(ctx, "git", args...); err != nil {
			return usecase.SourceCreation{BranchCreated: created || !existed}, err
		}
		created = created || !existed
	}
	return usecase.SourceCreation{BranchCreated: created}, nil
}

func (a GitSourceAdapter) worktreePath(source domain.Source) string {
	if filepath.IsAbs(source.Path) || a.Root == "" {
		return source.Path
	}
	return filepath.Join(a.Root, source.Path)
}

func (a GitSourceAdapter) gitArgs(source domain.Source) []string {
	repository := source.TemplatePath
	if repository == "" {
		repository = "."
	}
	if !filepath.IsAbs(repository) && a.Root != "" {
		repository = filepath.Join(a.Root, repository)
	}
	return []string{"-C", repository}
}

func (a GitSourceAdapter) branchExists(ctx context.Context, source domain.Source) (bool, error) {
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

func (a GitSourceAdapter) ResumeSource(ctx context.Context, cell domain.Cell) error {
	for _, source := range cell.Sources {
		if err := a.resumeSource(ctx, source); err != nil {
			return err
		}
	}
	return nil
}

func (a GitSourceAdapter) resumeSource(ctx context.Context, source domain.Source) error {
	path := a.worktreePath(source)
	if _, err := os.Stat(filepath.Join(path, ".git")); err == nil {
		branch, branchErr := a.Runner.Output(ctx, "git", "-C", path, "branch", "--show-current")
		if branchErr != nil {
			return fmt.Errorf("inspect existing worktree %q: %w", source.Path, branchErr)
		}
		if strings.TrimSpace(branch) != source.Branch {
			return fmt.Errorf("worktree %q uses branch %q, want %q", source.Path, strings.TrimSpace(branch), source.Branch)
		}
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect worktree %q: %w", source.Path, err)
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

func (a GitSourceAdapter) CleanSource(ctx context.Context, cell domain.Cell) error {
	var cleanErr error
	for _, source := range cell.Sources {
		args := append(a.gitArgs(source), "worktree", "remove", "--force", a.worktreePath(source))
		if err := a.Runner.Run(ctx, "git", args...); err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "is not a working tree") {
				err = fmt.Errorf("%w: %v", domain.ErrNotFound, err)
			}
			cleanErr = errors.Join(cleanErr, err)
		}
	}
	return cleanErr
}
