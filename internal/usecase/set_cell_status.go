package usecase

import (
	"context"
	"fmt"
)
import "github.com/hgsg11/paracell/internal/domain"

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
			if domain.CellMatches(cell, input.Cell) {
				changed, setErr := domain.SetCellStatus(cell, input.Status)
				if setErr != nil {
					return nil, setErr
				}
				cells[i] = changed
				updated = changed
				return cells, nil
			}
		}
		return nil, fmt.Errorf("cell %q not found", input.Cell)
	})
	if err != nil {
		return domain.Cell{}, err
	}
	updated = domain.CellAfterPersistence(updated)
	if input.Status == domain.Ready && u.NotificationFactory != nil {
		notifier, err := u.NotificationFactory.Notification(domain.CellResourceDrivers(updated).Notification)
		if err != nil {
			return domain.Cell{}, err
		}
		if err := notifier.NotifyReady(ctx, domain.SessionName(updated), "Ready: "+domain.CellName(updated)); err != nil {
			return domain.Cell{}, err
		}
	}
	return updated, nil
}
