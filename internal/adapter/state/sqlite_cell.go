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
	"github.com/hgsg11/paracell/internal/usecase"
	_ "modernc.org/sqlite"
)

type SQLiteCellAdapter struct{ Path string }

func NewSQLiteCellAdapter(path string) SQLiteCellAdapter {
	return SQLiteCellAdapter{Path: path}
}

func (a SQLiteCellAdapter) Initialize(ctx context.Context) error {
	db, err := a.open(ctx)
	if err != nil {
		return err
	}
	return db.Close()
}

func (a SQLiteCellAdapter) LoadCells(ctx context.Context) (usecase.CellSet, error) {
	db, err := a.open(ctx)
	if err != nil {
		return usecase.CellSet{}, err
	}
	defer db.Close()
	return loadCells(ctx, db)
}

func (a SQLiteCellAdapter) UpdateCells(ctx context.Context, update func(usecase.CellSet) (usecase.CellSet, error)) error {
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
	cloned, err := cloneCellSet(current)
	if err != nil {
		return err
	}
	next, err := update(cloned)
	if err != nil {
		return err
	}
	if err := validateCellSet(next); err != nil {
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

// SaveCells replaces the stored CellGroups and CommanderCells. Tests use it to
// prepare state; it removes every Target and Dependency Cell.
func (a SQLiteCellAdapter) SaveCells(ctx context.Context, groups []domain.CellGroup, commanders []domain.CommanderCell) error {
	return a.UpdateCells(ctx, func(usecase.CellSet) (usecase.CellSet, error) {
		return usecase.NewCellSet(groups, commanders, nil, nil), nil
	})
}

func (a SQLiteCellAdapter) open(ctx context.Context) (*sql.DB, error) {
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

// storedRow is one cells row with its record decoded into raw values.
type storedRow struct {
	ID      string
	Version uint64
	Record  cellRecord
}

// queryRows reads every cells row in list order and decodes all record formats.
func queryRows(ctx context.Context, queryer stateQueryer) ([]storedRow, error) {
	rows, err := queryer.QueryContext(ctx, "SELECT id, version, record FROM cells ORDER BY position")
	if err != nil {
		return nil, fmt.Errorf("query cells: %w", err)
	}
	defer rows.Close()
	var stored []storedRow
	for rows.Next() {
		var id string
		var version uint64
		var data []byte
		if err := rows.Scan(&id, &version, &data); err != nil {
			return nil, fmt.Errorf("scan CommanderCell: %w", err)
		}
		record, err := decodeCellRecord(data, version)
		if err != nil {
			return nil, fmt.Errorf("decode CommanderCell %q: %w", id, err)
		}
		if record.Commander.ID != id {
			return nil, fmt.Errorf("CommanderCell %q identity does not match its state row", id)
		}
		stored = append(stored, storedRow{ID: id, Version: version, Record: record})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate CommanderCells: %w", err)
	}
	return stored, nil
}

func loadCells(ctx context.Context, queryer stateQueryer) (usecase.CellSet, error) {
	rows, err := queryRows(ctx, queryer)
	if err != nil {
		return usecase.CellSet{}, err
	}
	set := usecase.NewCellSet(nil, nil, nil, nil)
	for _, row := range rows {
		group, commander, targets, dependencies, err := row.Record.restore(row.Version)
		if err != nil {
			return usecase.CellSet{}, fmt.Errorf("restore CommanderCell %q: %w", row.ID, err)
		}
		set.Groups = append(set.Groups, group)
		set.Commanders = append(set.Commanders, commander)
		set.Targets = append(set.Targets, targets...)
		set.Dependencies = append(set.Dependencies, dependencies...)
	}
	if err := validateCellSet(set); err != nil {
		return usecase.CellSet{}, fmt.Errorf("validate persisted Cells: %w", err)
	}
	return set, nil
}

// validateCellSet checks the associations by CellGroup: unique IDs, exactly one
// CommanderCell per CellGroup, and no Cell that refers to an unknown CellGroup.
func validateCellSet(set usecase.CellSet) error {
	groups := make(map[string]bool, len(set.Groups))
	for _, group := range set.Groups {
		if _, exists := groups[group.ID]; exists {
			return fmt.Errorf("duplicate CellGroup id %q", group.ID)
		}
		groups[group.ID] = false
	}
	commanders := make(map[string]struct{}, len(set.Commanders))
	for _, commander := range set.Commanders {
		if _, exists := commanders[commander.ID]; exists {
			return fmt.Errorf("duplicate CommanderCell id %q", commander.ID)
		}
		commanders[commander.ID] = struct{}{}
		hasCommander, exists := groups[commander.CellGroupID]
		if !exists {
			return fmt.Errorf("CommanderCell %q has unknown CellGroup %q", commander.ID, commander.CellGroupID)
		}
		if hasCommander {
			return fmt.Errorf("CellGroup %q has more than one CommanderCell", commander.CellGroupID)
		}
		groups[commander.CellGroupID] = true
	}
	for id, hasCommander := range groups {
		if !hasCommander {
			return fmt.Errorf("CellGroup %q has no CommanderCell", id)
		}
	}
	targets := make(map[string]struct{}, len(set.Targets))
	for _, target := range set.Targets {
		if _, exists := groups[target.CellGroupID]; !exists {
			return fmt.Errorf("TargetCell %q has unknown CellGroup %q", target.ID, target.CellGroupID)
		}
		if _, exists := targets[target.ID]; exists {
			return fmt.Errorf("duplicate TargetCell id %q", target.ID)
		}
		targets[target.ID] = struct{}{}
	}
	dependencies := make(map[string]struct{}, len(set.Dependencies))
	for _, dependency := range set.Dependencies {
		if _, exists := groups[dependency.CellGroupID]; !exists {
			return fmt.Errorf("DependencyCell %q has unknown CellGroup %q", dependency.ID, dependency.CellGroupID)
		}
		if _, exists := dependencies[dependency.ID]; exists {
			return fmt.Errorf("duplicate DependencyCell id %q", dependency.ID)
		}
		dependencies[dependency.ID] = struct{}{}
	}
	return nil
}

type stateExecer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

// rowChange is the row a CellSet wants for one CellGroup, keyed by its
// CommanderCell ID. Version is the CommanderCell version the caller holds.
type rowChange struct {
	ID      string
	Issue   string
	Version domain.CellVersion
	Record  cellRecord
}

// applyChanges writes the difference between current and next. Only this
// adapter advances versions: an update runs "version = version + 1" guarded by
// the version the caller holds, so a stale CommanderCell matches no row and the
// transaction fails with ErrVersionConflict.
func applyChanges(ctx context.Context, execer stateExecer, current usecase.CellSet, next usecase.CellSet) error {
	currentRows, err := rowChanges(current)
	if err != nil {
		return err
	}
	nextRows, err := rowChanges(next)
	if err != nil {
		return err
	}
	currentByID := make(map[string]int, len(currentRows))
	for position, row := range currentRows {
		currentByID[row.ID] = position
	}
	nextByID := make(map[string]struct{}, len(nextRows))
	for _, row := range nextRows {
		nextByID[row.ID] = struct{}{}
	}
	// Release deleted positions before compacting the surviving groups.
	for _, row := range currentRows {
		if _, exists := nextByID[row.ID]; exists {
			continue
		}
		result, err := execer.ExecContext(ctx, "DELETE FROM cells WHERE id = ? AND version = ?", row.ID, uint64(row.Version))
		if err != nil {
			return fmt.Errorf("delete CommanderCell %q: %w", row.ID, err)
		}
		if err := requireOneRow(result, "delete", row.ID, row.Version); err != nil {
			return err
		}
	}
	for position, row := range nextRows {
		storedPosition, exists := currentByID[row.ID]
		if !exists {
			if row.Version != 0 {
				return fmt.Errorf("new CommanderCell %q must not have a version, found %d", row.ID, row.Version)
			}
			if err := insertCell(ctx, execer, position, row); err != nil {
				return err
			}
			continue
		}
		stored := currentRows[storedPosition]
		if storedPosition == position && reflect.DeepEqual(stored.Record, row.Record) {
			continue
		}
		if err := writeCell(ctx, execer, position, row); err != nil {
			return err
		}
	}
	return nil
}

// rowChanges builds one row per CellGroup in list order.
func rowChanges(set usecase.CellSet) ([]rowChange, error) {
	if err := validateCellSet(set); err != nil {
		return nil, err
	}
	rows := make([]rowChange, 0, len(set.Groups))
	for _, group := range set.Groups {
		var commander domain.CommanderCell
		for _, candidate := range set.Commanders {
			if candidate.CellGroupID == group.ID {
				commander = candidate
				break
			}
		}
		targets, dependencies := domain.SelectCellGroupMembersService(group.ID, set.Targets, set.Dependencies)
		rows = append(rows, rowChange{ID: commander.ID, Issue: group.Issue, Version: commander.Version, Record: newCellRecord(group, commander, targets, dependencies)})
	}
	return rows, nil
}

func writeCell(ctx context.Context, execer stateExecer, position int, row rowChange) error {
	data, err := json.Marshal(row.Record)
	if err != nil {
		return fmt.Errorf("encode CommanderCell %q: %w", row.ID, err)
	}
	result, err := execer.ExecContext(ctx, "UPDATE cells SET issue = ?, position = ?, version = version + 1, record = ? WHERE id = ? AND version = ?",
		row.Issue, position, data, row.ID, uint64(row.Version))
	if err != nil {
		return fmt.Errorf("update CommanderCell %q: %w", row.ID, err)
	}
	return requireOneRow(result, "update", row.ID, row.Version)
}

func insertCell(ctx context.Context, execer stateExecer, position int, row rowChange) error {
	data, err := json.Marshal(row.Record)
	if err != nil {
		return fmt.Errorf("encode CommanderCell %q: %w", row.ID, err)
	}
	if _, err := execer.ExecContext(ctx, "INSERT INTO cells (id, issue, position, version, record) VALUES (?, ?, ?, 1, ?)",
		row.ID, row.Issue, position, data); err != nil {
		return fmt.Errorf("insert CommanderCell %q: %w", row.ID, err)
	}
	return nil
}

func requireOneRow(result sql.Result, operation string, id string, version domain.CellVersion) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect %s CommanderCell %q: %w", operation, id, err)
	}
	if affected != 1 {
		return fmt.Errorf("%w: %s CommanderCell %q expected version %d", domain.ErrVersionConflict, operation, id, version)
	}
	return nil
}

// cloneCellSet copies a loaded set so that the update callback cannot change it.
func cloneCellSet(set usecase.CellSet) (usecase.CellSet, error) {
	cloned := usecase.NewCellSet(set.Groups, set.Commanders, nil, nil)
	for i := range cloned.Commanders {
		cloned.Commanders[i].Workspace = domain.NewWorkspace(cloned.Commanders[i].Workspace.Driver, cloned.Commanders[i].Workspace.Windows)
	}
	for _, target := range set.Targets {
		copy, err := domain.RestoreTargetCell(target)
		if err != nil {
			return usecase.CellSet{}, err
		}
		cloned.Targets = append(cloned.Targets, copy)
	}
	for _, dependency := range set.Dependencies {
		copy, err := domain.RestoreDependencyCell(dependency)
		if err != nil {
			return usecase.CellSet{}, err
		}
		cloned.Dependencies = append(cloned.Dependencies, copy)
	}
	return cloned, nil
}
