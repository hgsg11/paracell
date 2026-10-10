package usecase

import (
	"context"
	"fmt"

	"github.com/hgsg11/paracell/internal/domain"
)

type SetCellStatusInput struct {
	Cell   string
	Status domain.CellStatus
}

type SetCellStatusUseCase struct {
	Cells               CellPort
	NotificationFactory NotificationProviderFactory
}

func (u SetCellStatusUseCase) Execute(ctx context.Context, input SetCellStatusInput) (domain.CommanderCell, error) {
	var updated domain.CommanderCell
	var group domain.CellGroup
	err := u.Cells.UpdateCells(ctx, func(cells CellSet) (CellSet, error) {
		cell, ok := cells.FindCommander(input.Cell)
		if !ok {
			return CellSet{}, fmt.Errorf("cell %q not found", input.Cell)
		}
		if err := cell.SetStatus(input.Status); err != nil {
			return CellSet{}, err
		}
		for i := range cells.Commanders {
			if cells.Commanders[i].ID == cell.ID {
				cells.Commanders[i] = cell
				break
			}
		}
		group, ok = cells.CellGroup(cell.CellGroupID)
		if !ok {
			return CellSet{}, fmt.Errorf("CellGroup %q not found", cell.CellGroupID)
		}
		updated = cell
		return cells, nil
	})
	if err != nil {
		return domain.CommanderCell{}, err
	}
	if err := updated.AdvanceVersion(); err != nil {
		return domain.CommanderCell{}, err
	}
	if input.Status == domain.Ready && u.NotificationFactory != nil {
		notifier, err := u.NotificationFactory.Notification(group.NotificationDriver)
		if err != nil {
			return domain.CommanderCell{}, err
		}
		if err := notifier.NotifyReady(ctx, group.WorkspaceName(), "Ready: "+group.Name().Value); err != nil {
			return domain.CommanderCell{}, err
		}
	}
	return updated, nil
}
