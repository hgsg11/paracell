package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/hgsg11/paracell/internal/domain"
)

type AnnotateCellInput struct {
	Cell string
	Note string
}

type AnnotateCellUseCase struct {
	State   CellStatePort
	Session SessionPort
}

func (u AnnotateCellUseCase) Execute(ctx context.Context, input AnnotateCellInput) (domain.Cell, error) {
	note, err := domain.NormalizeCellNote(input.Note)
	if err != nil {
		return domain.Cell{}, err
	}

	var updated domain.Cell
	if err := u.State.UpdateCells(ctx, func(cells []domain.Cell) ([]domain.Cell, error) {
		for i, cell := range cells {
			if cell.ID == input.Cell || cell.Issue == input.Cell || cell.Name == input.Cell {
				cell.Note = note
				cells[i] = cell
				updated = cell
				return cells, nil
			}
		}
		return nil, fmt.Errorf("cell %q not found", input.Cell)
	}); err != nil {
		return domain.Cell{}, err
	}

	if u.Session == nil {
		return updated, nil
	}
	if err := u.Session.UpdateStatusLabel(ctx, updated); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return updated, nil
		}
		return updated, fmt.Errorf("cell note was saved, but tmux status label update failed: %w", err)
	}
	return updated, nil
}
