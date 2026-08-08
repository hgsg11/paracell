package source

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hgsg11/paracell/internal/adapter/system"
	"github.com/hgsg11/paracell/internal/domain"
	"github.com/hgsg11/paracell/internal/usecase"
)

type GitSourceAdapter struct {
	Runner system.Runner
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

func (a GitSourceAdapter) RollbackSource(ctx context.Context, cell domain.Cell, creation usecase.SourceCreation) error {
	var rollbackErr error
	if err := a.CleanSource(ctx, cell); err != nil && !errors.Is(err, domain.ErrNotFound) {
		rollbackErr = errors.Join(rollbackErr, err)
	}
	if creation.BranchCreated {
		if err := a.Runner.Run(ctx, "git", "branch", "-D", cell.Branch); err != nil && !isMissingBranchError(err) {
			rollbackErr = errors.Join(rollbackErr, err)
		}
	}
	return rollbackErr
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

func isMissingBranchError(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "branch") && (strings.Contains(message, "not found") || strings.Contains(message, "not exist"))
}
