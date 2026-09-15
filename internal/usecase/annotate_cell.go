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
	State          CellStatePort
	SessionFactory SessionProviderFactory
}

func (u AnnotateCellUseCase) Execute(ctx context.Context, input AnnotateCellInput) (domain.Cell, error) {
	note, err := domain.NormalizeCellNote(input.Note)
	if err != nil {
		return domain.Cell{}, err
	}

	var updated domain.Cell
	if err := u.State.UpdateCells(ctx, func(cells []domain.Cell) ([]domain.Cell, error) {
		for i, cell := range cells {
			if domain.CellMatches(cell, input.Cell) {
				cell, err = domain.SetCellNote(cell, note)
				if err != nil {
					return nil, err
				}
				cells[i] = cell
				updated = cell
				return cells, nil
			}
		}
		return nil, fmt.Errorf("cell %q not found", input.Cell)
	}); err != nil {
		return domain.Cell{}, err
	}
	updated = domain.CellAfterPersistence(updated)

	if u.SessionFactory == nil {
		return updated, nil
	}
	session, err := u.SessionFactory.Session(domain.CellResourceDrivers(updated).Session)
	if err != nil {
		return updated, err
	}
	if err := domain.UpdateSessionStatusLabel(ctx, updated, session); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return updated, nil
		}
		return updated, fmt.Errorf("cell note was saved, but tmux status label update failed: %w", err)
	}
	return updated, nil
}
