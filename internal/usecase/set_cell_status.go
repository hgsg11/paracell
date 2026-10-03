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
	err := u.Cells.UpdateCells(ctx, func(cells CellSet) (CellSet, error) {
		for i, cell := range cells.Commanders {
			if cell.Matches(input.Cell) {
				if setErr := cell.SetStatus(input.Status); setErr != nil {
					return CellSet{}, setErr
				}
				cells.Commanders[i] = cell
				updated = cell
				return cells, nil
			}
		}
		return CellSet{}, fmt.Errorf("cell %q not found", input.Cell)
	})
	if err != nil {
		return domain.CommanderCell{}, err
	}
	if err := updated.AdvanceVersion(); err != nil {
		return domain.CommanderCell{}, err
	}
	if input.Status == domain.Ready && u.NotificationFactory != nil {
		notifier, err := u.NotificationFactory.Notification(updated.ResourceDrivers().Notification)
		if err != nil {
			return domain.CommanderCell{}, err
		}
		if err := notifier.NotifyReady(ctx, updated.WorkspaceName(), "Ready: "+updated.Name().Value); err != nil {
			return domain.CommanderCell{}, err
		}
	}
	return updated, nil
}
