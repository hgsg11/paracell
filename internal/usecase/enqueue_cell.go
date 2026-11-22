package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/hgsg11/paracell/internal/domain"
)

type EnqueueCellInput struct {
	Issue    string
	Template string
	Prefix   string
	Command  string
	Note     *string
	Modify   []string
	Create   []string
}

// EnqueueCellOutput tells whether the entry started at once and, when it did
// not, the issues of the entries it waits for.
type EnqueueCellOutput struct {
	Entry     domain.QueueEntry
	Started   bool
	BlockedBy []string
}

type EnqueueCellUseCase struct {
	Config ConfigPort
	Cells  CellPort
	Files  WorkspaceFilePort
	IDs    IDGenerator
	// Fork starts the entries that do not wait for anyone.
	Fork ForkCellUseCase
}

// Execute validates the declaration, adds it to the Queue, and starts every
// entry that no longer waits, the new one included.
func (u EnqueueCellUseCase) Execute(ctx context.Context, input EnqueueCellInput) (EnqueueCellOutput, error) {
	if input.Issue == "" {
		return EnqueueCellOutput{}, fmt.Errorf("issue is required")
	}
	// Reject what fork would reject at start, before the entry waits.
	cfg, err := u.Config.Load(ctx)
	if err != nil {
		return EnqueueCellOutput{}, err
	}
	prefix := input.Prefix
	if prefix == "" {
		prefix = domain.FeatPrefix
	}
	if _, err := cfg.BranchPrefix(prefix); err != nil {
		return EnqueueCellOutput{}, err
	}
	name := domain.NewCellName(input.Issue)
	resolved, err := cfg.Resolve(input.Template, domain.NewTemplateVars(input.Issue, name.Value, cfg.ProjectName, input.Command))
	if err != nil {
		return EnqueueCellOutput{}, err
	}
	if resolved.Commander == nil {
		return EnqueueCellOutput{}, fmt.Errorf("template %q does not define a CommanderCell", input.Template)
	}
	plan, err := newChangePlan(input.Modify, input.Create)
	if err != nil {
		return EnqueueCellOutput{}, err
	}
	note := ""
	if input.Note != nil {
		if note, err = domain.NormalizeCellNote(*input.Note); err != nil {
			return EnqueueCellOutput{}, err
		}
	}
	request, err := domain.NewForkRequest(input.Template, input.Prefix, input.Command, note)
	if err != nil {
		return EnqueueCellOutput{}, err
	}
	existing, err := u.Files.ExistingPaths(ctx, plan.Paths())
	if err != nil {
		return EnqueueCellOutput{}, err
	}
	entryID := u.IDs.NewID()
	var entry domain.QueueEntry
	if err := u.Cells.UpdateCells(ctx, func(latest CellSet) (CellSet, error) {
		if err := domain.EnsureCommanderCellUniqueService(latest.Groups, input.Issue, name); err != nil {
			return CellSet{}, err
		}
		entry, err = latest.Queue.Enqueue(entryID, input.Issue, plan, request, existing)
		if err != nil {
			return CellSet{}, err
		}
		return latest, nil
	}); err != nil {
		return EnqueueCellOutput{}, err
	}
	_, startErr := (queueStarter{Cells: u.Cells, Fork: u.Fork}).StartReleased(ctx)
	output := EnqueueCellOutput{Entry: entry}
	set, err := u.Cells.LoadCells(ctx)
	if err != nil {
		return output, errors.Join(startErr, err)
	}
	if _, running := domain.FindQueueEntryCellGroupService(set.Groups, entry.ID); running {
		output.Started = true
	} else {
		for _, blocker := range set.Queue.Blockers(entry.ID) {
			output.BlockedBy = append(output.BlockedBy, blocker.Issue)
		}
	}
	return output, startErr
}

func newChangePlan(modify []string, create []string) (domain.ChangePlan, error) {
	var errs error
	toPaths := func(values []string) []domain.WorkspacePath {
		paths := make([]domain.WorkspacePath, 0, len(values))
		for _, value := range values {
			path, err := domain.NewWorkspacePath(value)
			if err != nil {
				errs = errors.Join(errs, err)
				continue
			}
			paths = append(paths, path)
		}
		return paths
	}
	modifyPaths, createPaths := toPaths(modify), toPaths(create)
	if errs != nil {
		return domain.ChangePlan{}, errs
	}
	return domain.NewChangePlan(modifyPaths, createPaths)
}
