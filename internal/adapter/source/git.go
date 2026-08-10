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
	if cell.BranchMode == domain.RepositoryBranchModeRequire {
		return usecase.SourceCreation{}, a.Runner.Run(ctx, "git", "worktree", "add", cell.Source.Path, cell.Branch)
	}
	branchExisted, err := a.branchExists(ctx, cell.Branch)
	if err != nil {
		return usecase.SourceCreation{}, err
	}
	if cell.BranchMode == domain.RepositoryBranchModeReuse && branchExisted {
		return usecase.SourceCreation{}, a.Runner.Run(ctx, "git", "worktree", "add", cell.Source.Path, cell.Branch)
	}
	args := []string{"worktree", "add", cell.Source.Path, "-b", cell.Branch}
	if cell.Base != "" && cell.Base != "current" {
		args = append(args, cell.Base)
	}
	if err := a.Runner.Run(ctx, "git", args...); err != nil {
		creation := usecase.SourceCreation{}
		if !branchExisted {
			existsAfterFailure, checkErr := a.branchExists(context.WithoutCancel(ctx), cell.Branch)
			if checkErr == nil {
				creation.BranchCreated = existsAfterFailure
			}
		}
		return creation, err
	}
	return usecase.SourceCreation{BranchCreated: !branchExisted}, nil
}

func (a GitSourceAdapter) branchExists(ctx context.Context, branch string) (bool, error) {
	err := a.Runner.Run(ctx, "git", "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	if err == nil {
		return true, nil
	}
	var exitCoder interface{ ExitCode() int }
	if errors.As(err, &exitCoder) && exitCoder.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("check git branch %q: %w", branch, err)
}

func (a GitSourceAdapter) ResumeSource(ctx context.Context, cell domain.Cell) error {
	path := cell.Source.Path
	if !filepath.IsAbs(path) && a.Root != "" {
		path = filepath.Join(a.Root, path)
	}
	if _, err := os.Stat(filepath.Join(path, ".git")); err == nil {
		branch, branchErr := a.Runner.Output(ctx, "git", "-C", cell.Source.Path, "branch", "--show-current")
		if branchErr != nil {
			return fmt.Errorf("inspect existing worktree %q: %w", cell.Source.Path, branchErr)
		}
		if strings.TrimSpace(branch) != cell.Branch {
			return fmt.Errorf("worktree %q uses branch %q, want %q", cell.Source.Path, strings.TrimSpace(branch), cell.Branch)
		}
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect worktree %q: %w", cell.Source.Path, err)
	}
	if info, err := os.Stat(path); err == nil {
		if !info.IsDir() {
			return fmt.Errorf("worktree path %q exists and is not a directory", cell.Source.Path)
		}
		entries, readErr := os.ReadDir(path)
		if readErr != nil {
			return fmt.Errorf("inspect partial worktree %q: %w", cell.Source.Path, readErr)
		}
		if len(entries) != 0 {
			return fmt.Errorf("refusing to replace non-empty partial worktree %q", cell.Source.Path)
		}
		if removeErr := os.Remove(path); removeErr != nil {
			return fmt.Errorf("remove empty partial worktree %q: %w", cell.Source.Path, removeErr)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect partial worktree %q: %w", cell.Source.Path, err)
	}

	exists, err := a.branchExists(ctx, cell.Branch)
	if err != nil {
		return err
	}
	if exists {
		return a.Runner.Run(ctx, "git", "worktree", "add", cell.Source.Path, cell.Branch)
	}
	_, err = a.CreateSource(ctx, cell)
	return err
}

func (a GitSourceAdapter) CleanSource(ctx context.Context, cell domain.Cell) error {
	err := a.Runner.Run(ctx, "git", "worktree", "remove", "--force", cell.Source.Path)
	if err == nil {
		return nil
	}
	if strings.Contains(strings.ToLower(err.Error()), "is not a working tree") {
		return fmt.Errorf("%w: %v", domain.ErrNotFound, err)
	}
	return err
}
