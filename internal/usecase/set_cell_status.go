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
	State    CellStatePort
	Notifier Notifier
}

func (u SetCellStatusUseCase) Execute(ctx context.Context, input SetCellStatusInput) (domain.Cell, error) {
	var updated domain.Cell
	err := u.State.UpdateCells(ctx, func(cells []domain.Cell) ([]domain.Cell, error) {
		for i, cell := range cells {
			if cell.ID == input.Cell || cell.Issue == input.Cell || cell.Name == input.Cell {
				if err := cell.SetStatus(input.Status); err != nil {
					return nil, err
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
	if input.Status == domain.Ready && u.Notifier != nil {
		if err := u.Notifier.NotifyReady(ctx, updated, "Ready: "+updated.Name); err != nil {
			return domain.Cell{}, err
		}
	}
	return updated, nil
}
