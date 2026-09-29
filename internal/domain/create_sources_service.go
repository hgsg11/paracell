package domain

import (
	"context"
	"fmt"
)

type SourceCreationPort interface {
	CreateSource(ctx context.Context, repository string, worktree string, base string, branch string) error
}

type SourceCellSavePort interface {
	SaveCell(ctx context.Context, cell Cell) error
}

func CreateSourcesService(ctx context.Context, cell *Cell, port SourceCreationPort, cells SourceCellSavePort) error {
	fail := func(err error) error {
		if stateErr := cell.FailCreation(CreationStageSource, err); stateErr != nil {
			return fmt.Errorf("%w; record source creation failure: %v", err, stateErr)
		}
		if saveErr := cells.SaveCell(ctx, *cell); saveErr != nil {
			return fmt.Errorf("%w; save source creation failure: %v", err, saveErr)
		}
		return err
	}
	if err := cell.SetCreationStage(CreationStageSource); err != nil {
		return err
	}
	if err := cells.SaveCell(ctx, *cell); err != nil {
		return err
	}
	for _, source := range cell.Sources.Items {
		if err := port.CreateSource(ctx, source.Path, source.Worktree, source.Base, source.Branch); err != nil {
			return fail(err)
		}
	}
	return cells.SaveCell(ctx, *cell)
}
