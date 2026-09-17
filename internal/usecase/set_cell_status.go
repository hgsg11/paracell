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
	State               CellStatePort
	NotificationFactory NotificationProviderFactory
}

func (u SetCellStatusUseCase) Execute(ctx context.Context, input SetCellStatusInput) (domain.Cell, error) {
	var updated domain.Cell
	err := u.State.UpdateCells(ctx, func(cells []domain.Cell) ([]domain.Cell, error) {
		for i, cell := range cells {
			if cell.Matches(input.Cell) {
				if setErr := cell.SetStatus(input.Status); setErr != nil {
					return nil, setErr
				}
				cells[i] = cell
				updated = cell
				return cells, nil
			}
		}
		return nil, fmt.Errorf("cell %q not found", input.Cell)
	})
	if err != nil {
		return domain.Cell{}, err
	}
	updated.AdvanceVersion()
	if input.Status == domain.Ready && u.NotificationFactory != nil {
		notifier, err := u.NotificationFactory.Notification(updated.ResourceDrivers().Notification)
		if err != nil {
			return domain.Cell{}, err
		}
		if err := notifier.NotifyReady(ctx, updated.SessionName(), "Ready: "+updated.Name().Value); err != nil {
			return domain.Cell{}, err
		}
	}
	return updated, nil
}
