package state

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/hgsg11/paracell/internal/domain"
)

type JSONCellStateAdapter struct {
	Path string
}

type stateFile struct {
	Cells []domain.Cell `json:"cells"`
}

func (a JSONCellStateAdapter) LoadCells(ctx context.Context) ([]domain.Cell, error) {
	_ = ctx
	data, err := os.ReadFile(a.Path)
	if errors.Is(err, os.ErrNotExist) {
		return []domain.Cell{}, nil
	}
	if err != nil {
		return nil, err
	}
	var file stateFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, err
	}
	return file.Cells, nil
}

func (a JSONCellStateAdapter) SaveCells(ctx context.Context, cells []domain.Cell) error {
	_ = ctx
	if err := os.MkdirAll(filepath.Dir(a.Path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(stateFile{Cells: cells}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(a.Path, append(data, '\n'), 0o644)
}
