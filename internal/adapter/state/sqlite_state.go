package state

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/hgsg11/paracell/internal/domain"
	_ "modernc.org/sqlite"
)

type SQLiteCellStateAdapter struct{ Path string }

func NewSQLiteCellStateAdapter(path string) SQLiteCellStateAdapter {
	return SQLiteCellStateAdapter{Path: path}
}

func (a SQLiteCellStateAdapter) Initialize(ctx context.Context) error {
	db, err := a.open(ctx)
	if err != nil {
		return err
	}
	return db.Close()
}

func (a SQLiteCellStateAdapter) LoadCells(ctx context.Context) ([]domain.Cell, error) {
	db, err := a.open(ctx)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return loadCells(ctx, db)
}

func (a SQLiteCellStateAdapter) UpdateCells(ctx context.Context, update func([]domain.Cell) ([]domain.Cell, error)) error {
	db, err := a.open(ctx)
	if err != nil {
		return err
	}
	defer db.Close()
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("open state connection: %w", err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return fmt.Errorf("begin state update: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
		}
	}()
	current, err := loadCells(ctx, conn)
	if err != nil {
		return err
	}
	next, err := update(cloneCells(current))
	if err != nil {
		return err
	}
	if err := applyChanges(ctx, conn, current, next); err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return fmt.Errorf("commit state update: %w", err)
	}
	committed = true
	return nil
}

func (a SQLiteCellStateAdapter) DeleteCell(ctx context.Context, cell domain.Cell) error {
	record := cell.Stored()
	db, err := a.open(ctx)
	if err != nil {
		return err
	}
	defer db.Close()
	result, err := db.ExecContext(ctx, "DELETE FROM cells WHERE id = ? AND version = ?", record.ID, record.Version)
	if err != nil {
		return fmt.Errorf("delete cell %q: %w", record.ID, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect deleted cell %q: %w", record.ID, err)
	}
	if affected != 1 {
		return fmt.Errorf("%w: delete cell %q expected version %d", domain.ErrVersionConflict, record.ID, record.Version)
	}
	return nil
}

func (a SQLiteCellStateAdapter) SaveCells(ctx context.Context, cells []domain.Cell) error {
	return a.UpdateCells(ctx, func([]domain.Cell) ([]domain.Cell, error) {
		return cloneCells(cells), nil
	})
}

func (a SQLiteCellStateAdapter) open(ctx context.Context) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(a.Path), 0o755); err != nil {
		return nil, fmt.Errorf("create state directory: %w", err)
	}
	db, err := sql.Open("sqlite", a.Path)
	if err != nil {
		return nil, fmt.Errorf("open state database: %w", err)
	}
	db.SetMaxOpenConns(1)
	for _, statement := range []string{"PRAGMA busy_timeout = 10000", "PRAGMA foreign_keys = ON"} {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			db.Close()
			return nil, fmt.Errorf("configure state database: %w", err)
		}
	}
	if _, err := db.ExecContext(ctx, "PRAGMA journal_mode = WAL"); err != nil && !isSQLiteContention(err) {
		db.Close()
		return nil, fmt.Errorf("enable state WAL: %w", err)
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS cells (
		id TEXT PRIMARY KEY,
		issue TEXT NOT NULL UNIQUE,
		position INTEGER NOT NULL UNIQUE,
		version INTEGER NOT NULL,
		record BLOB NOT NULL
	)`); err != nil {
		db.Close()
		return nil, fmt.Errorf("initialize state schema: %w", err)
	}
	return db, nil
}

func isSQLiteContention(err error) bool {
	var sqliteErr interface{ Code() int }
	if !errors.As(err, &sqliteErr) {
		return false
	}
	code := sqliteErr.Code() & 0xff
	return code == 5 || code == 6
}

type stateQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func loadCells(ctx context.Context, queryer stateQueryer) ([]domain.Cell, error) {
	rows, err := queryer.QueryContext(ctx, "SELECT id, version, record FROM cells ORDER BY position")
	if err != nil {
		return nil, fmt.Errorf("query cells: %w", err)
	}
	defer rows.Close()
	cells := []domain.Cell{}
	for rows.Next() {
		var id string
		var version uint64
		var data []byte
		if err := rows.Scan(&id, &version, &data); err != nil {
			return nil, fmt.Errorf("scan cell: %w", err)
		}
		var stored domain.StoredCell
		if err := json.Unmarshal(data, &stored); err != nil {
			return nil, fmt.Errorf("decode cell %q: %w", id, err)
		}
		if stored.ID != id || stored.Version != version {
			return nil, fmt.Errorf("cell %q identity or version does not match its state row", id)
		}
		cell, err := domain.RestoreCell(stored)
		if err != nil {
			return nil, fmt.Errorf("restore cell %q: %w", id, err)
		}
		cells = append(cells, cell)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cells: %w", err)
	}
	return cells, nil
}

type stateExecer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func applyChanges(ctx context.Context, execer stateExecer, current []domain.Cell, next []domain.Cell) error {
	currentByID := make(map[string]domain.Cell, len(current))
	nextByID := make(map[string]domain.Cell, len(next))
	for _, cell := range current {
		record := cell.Stored()
		currentByID[record.ID] = cell
	}
	for position, cell := range next {
		record := cell.Stored()
		if _, exists := nextByID[record.ID]; exists {
			return fmt.Errorf("duplicate cell id %q", record.ID)
		}
		nextByID[record.ID] = cell
		stored, exists := currentByID[record.ID]
		if !exists {
			if record.Version != 1 {
				return fmt.Errorf("new cell %q must have version 1", record.ID)
			}
			if err := insertCell(ctx, execer, position, cell); err != nil {
				return err
			}
			continue
		}
		storedRecord := stored.Stored()
		if reflect.DeepEqual(storedRecord, record) && position == cellPosition(current, record.ID) {
			continue
		}
		if record.Version != storedRecord.Version {
			return fmt.Errorf("%w: update cell %q expected version %d, found %d", domain.ErrVersionConflict, record.ID, record.Version, storedRecord.Version)
		}
		persisted := cell
		if err := persisted.AdvanceVersion(); err != nil {
			return err
		}
		persistedRecord := persisted.Stored()
		data, err := json.Marshal(persistedRecord)
		if err != nil {
			return fmt.Errorf("encode cell %q: %w", record.ID, err)
		}
		result, err := execer.ExecContext(ctx, "UPDATE cells SET issue = ?, position = ?, version = ?, record = ? WHERE id = ? AND version = ?",
			persistedRecord.Issue, position, persistedRecord.Version, data, persistedRecord.ID, record.Version)
		if err != nil {
			return fmt.Errorf("update cell %q: %w", record.ID, err)
		}
		if err := requireOneRow(result, "update", record.ID, record.Version); err != nil {
			return err
		}
	}
	for _, cell := range current {
		record := cell.Stored()
		if _, exists := nextByID[record.ID]; exists {
			continue
		}
		result, err := execer.ExecContext(ctx, "DELETE FROM cells WHERE id = ? AND version = ?", record.ID, record.Version)
		if err != nil {
			return fmt.Errorf("delete cell %q: %w", record.ID, err)
		}
		if err := requireOneRow(result, "delete", record.ID, record.Version); err != nil {
			return err
		}
	}
	return nil
}

func insertCell(ctx context.Context, execer stateExecer, position int, cell domain.Cell) error {
	record := cell.Stored()
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encode cell %q: %w", record.ID, err)
	}
	if _, err := execer.ExecContext(ctx, "INSERT INTO cells (id, issue, position, version, record) VALUES (?, ?, ?, ?, ?)",
		record.ID, record.Issue, position, record.Version, data); err != nil {
		return fmt.Errorf("insert cell %q: %w", record.ID, err)
	}
	return nil
}

func requireOneRow(result sql.Result, operation string, id string, version uint64) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect %s cell %q: %w", operation, id, err)
	}
	if affected != 1 {
		return fmt.Errorf("%w: %s cell %q expected version %d", domain.ErrVersionConflict, operation, id, version)
	}
	return nil
}

func cellPosition(cells []domain.Cell, id string) int {
	for position, cell := range cells {
		if cell.Stored().ID == id {
			return position
		}
	}
	return -1
}

func cloneCells(cells []domain.Cell) []domain.Cell {
	cloned := make([]domain.Cell, len(cells))
	for i, cell := range cells {
		cloned[i] = cell.Clone()
	}
	return cloned
}
