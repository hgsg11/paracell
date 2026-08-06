package state

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hgsg11/paracell/internal/domain"
	_ "modernc.org/sqlite"
)

type SQLiteCellStateAdapter struct {
	Path string
}

const stateSchemaVersion = 1

func (a SQLiteCellStateAdapter) LoadCells(ctx context.Context) ([]domain.Cell, error) {
	db, err := a.open(ctx)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.QueryContext(ctx, `SELECT data FROM cells ORDER BY position`)
	if err != nil {
		return nil, fmt.Errorf("query cells: %w", err)
	}
	defer rows.Close()
	return decodeCells(rows)
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
	if _, err := conn.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
		return fmt.Errorf("begin state update: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.Background(), `ROLLBACK`)
		}
	}()

	rows, err := conn.QueryContext(ctx, `SELECT data FROM cells ORDER BY position`)
	if err != nil {
		return fmt.Errorf("query cells for update: %w", err)
	}
	cells, err := decodeCells(rows)
	closeErr := rows.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return fmt.Errorf("close cell rows: %w", closeErr)
	}
	next, err := update(cells)
	if err != nil {
		return err
	}
	if err := replaceCells(ctx, conn, next); err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, `COMMIT`); err != nil {
		return fmt.Errorf("commit state update: %w", err)
	}
	committed = true
	return nil
}

// SaveCells replaces the complete state transactionally. Production mutations
// should use UpdateCells so they always operate on the latest committed state.
func (a SQLiteCellStateAdapter) SaveCells(ctx context.Context, cells []domain.Cell) error {
	return a.UpdateCells(ctx, func([]domain.Cell) ([]domain.Cell, error) {
		return cells, nil
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
	if _, err := db.ExecContext(ctx, `PRAGMA busy_timeout = 10000`); err != nil {
		db.Close()
		return nil, fmt.Errorf("set state busy timeout: %w", err)
	}
	if _, err := db.ExecContext(ctx, `PRAGMA journal_mode = WAL`); err != nil && !isSQLiteContention(err) {
		db.Close()
		return nil, fmt.Errorf("enable state WAL: %w", err)
	}
	if err := a.initialize(ctx, db); err != nil {
		db.Close()
		return nil, err
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

func (a SQLiteCellStateAdapter) initialize(ctx context.Context, db *sql.DB) error {
	version, err := schemaVersion(ctx, db)
	if err != nil {
		return err
	}
	if version == stateSchemaVersion {
		return nil
	}
	if version > stateSchemaVersion {
		return fmt.Errorf("state schema version %d is newer than supported version %d", version, stateSchemaVersion)
	}

	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("open initialization connection: %w", err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
		return fmt.Errorf("begin state initialization: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.Background(), `ROLLBACK`)
		}
	}()
	version, err = schemaVersion(ctx, conn)
	if err != nil {
		return err
	}
	if version == stateSchemaVersion {
		if _, err := conn.ExecContext(ctx, `COMMIT`); err != nil {
			return fmt.Errorf("commit state initialization: %w", err)
		}
		committed = true
		return nil
	}

	statements := []string{
		`CREATE TABLE IF NOT EXISTS cells (
			id TEXT PRIMARY KEY,
			issue TEXT NOT NULL,
			name TEXT NOT NULL,
			position INTEGER NOT NULL,
			data BLOB NOT NULL
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS cells_issue_unique ON cells(issue) WHERE issue <> ''`,
		`CREATE UNIQUE INDEX IF NOT EXISTS cells_name_unique ON cells(name) WHERE name <> ''`,
	}
	for _, statement := range statements {
		if _, err := conn.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("initialize state schema: %w", err)
		}
	}
	if _, err := conn.ExecContext(ctx, `PRAGMA user_version = 1`); err != nil {
		return fmt.Errorf("record state schema version: %w", err)
	}

	if _, err := conn.ExecContext(ctx, `COMMIT`); err != nil {
		return fmt.Errorf("commit state initialization: %w", err)
	}
	committed = true
	return nil
}

type stateQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func schemaVersion(ctx context.Context, queryer stateQueryer) (int, error) {
	var version int
	if err := queryer.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		return 0, fmt.Errorf("read state schema version: %w", err)
	}
	return version, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func decodeCells(rows interface {
	Next() bool
	rowScanner
	Err() error
}) ([]domain.Cell, error) {
	cells := []domain.Cell{}
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, fmt.Errorf("scan cell: %w", err)
		}
		var cell domain.Cell
		if err := json.Unmarshal(data, &cell); err != nil {
			return nil, fmt.Errorf("decode cell: %w", err)
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

func replaceCells(ctx context.Context, execer stateExecer, cells []domain.Cell) error {
	if _, err := execer.ExecContext(ctx, `DELETE FROM cells`); err != nil {
		return fmt.Errorf("clear cells: %w", err)
	}
	for position, cell := range cells {
		data, err := json.Marshal(cell)
		if err != nil {
			return fmt.Errorf("encode cell %q: %w", cell.ID, err)
		}
		if _, err := execer.ExecContext(ctx,
			`INSERT INTO cells(id, issue, name, position, data) VALUES (?, ?, ?, ?, ?)`,
			cell.ID, cell.Issue, cell.Name, position, data,
		); err != nil {
			return fmt.Errorf("insert cell %q: %w", cell.ID, err)
		}
	}
	return nil
}
