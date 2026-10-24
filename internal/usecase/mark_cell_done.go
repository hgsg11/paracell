package usecase

import (
	"context"

	"github.com/hgsg11/paracell/internal/domain"
)

type MarkCellDoneInput struct {
	Cell string
}

type MarkCellDoneUseCase struct {
	Cells CellPort
}

// Execute marks the resolved Cell done. It fails when the Cell is already done.
func (u MarkCellDoneUseCase) Execute(ctx context.Context, input MarkCellDoneInput) (domain.CommanderCell, error) {
	commander, _, err := updateCommander(ctx, u.Cells, input.Cell, (*domain.CommanderCell).MarkDone)
	return commander, err
}

// updateCommander resolves a Cell, applies change to its CommanderCell, and saves
// it in one state update. It returns the CommanderCell as it was saved and its CellGroup.
func updateCommander(ctx context.Context, cells CellPort, identifier string, change func(*domain.CommanderCell) error) (domain.CommanderCell, domain.CellGroup, error) {
	var updated domain.CommanderCell
	var updatedGroup domain.CellGroup
	if err := cells.UpdateCells(ctx, func(set CellSet) (CellSet, error) {
		commander, group, err := set.resolve(identifier)
		if err != nil {
			return CellSet{}, err
		}
		if err := change(&commander); err != nil {
			return CellSet{}, err
		}
		if err := set.replaceCommander(commander); err != nil {
			return CellSet{}, err
		}
		updated, updatedGroup = commander, group
		return set, nil
	}); err != nil {
		return domain.CommanderCell{}, domain.CellGroup{}, err
	}
	return updated, updatedGroup, nil
}
