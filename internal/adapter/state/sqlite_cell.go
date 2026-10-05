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

func (a SQLiteCellAdapter) SaveCells(ctx context.Context, commanders []domain.CommanderCell) error {
	return a.UpdateCells(ctx, func(usecase.CellSet) (usecase.CellSet, error) {
		return usecase.NewCellSet(commanders, nil, nil), nil
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

type stateCellRecord struct {
	Commander    domain.StoredCommanderCell `json:"commander"`
	Targets      []domain.TargetCell        `json:"targets"`
	Dependencies []domain.DependencyCell    `json:"dependencies"`
}

func newStateCellRecord(commander domain.CommanderCell, targets []domain.TargetCell, dependencies []domain.DependencyCell) stateCellRecord {
	return stateCellRecord{Commander: commander.Stored(), Targets: append([]domain.TargetCell(nil), targets...), Dependencies: append([]domain.DependencyCell(nil), dependencies...)}
}

func loadCells(ctx context.Context, queryer stateQueryer) (usecase.CellSet, error) {
	rows, err := queryer.QueryContext(ctx, "SELECT id, version, record FROM cells ORDER BY position")
	if err != nil {
		return usecase.CellSet{}, fmt.Errorf("query cells: %w", err)
	}
	defer rows.Close()
	set := usecase.NewCellSet(nil, nil, nil)
	for rows.Next() {
		var id string
		var version uint64
		var data []byte
		if err := rows.Scan(&id, &version, &data); err != nil {
			return usecase.CellSet{}, fmt.Errorf("scan CommanderCell: %w", err)
		}
		record, err := decodeCellRecord(data)
		if err != nil {
			return usecase.CellSet{}, fmt.Errorf("decode CommanderCell %q: %w", id, err)
		}
		if record.Commander.ID != id || record.Commander.Version != version {
			return usecase.CellSet{}, fmt.Errorf("CommanderCell %q identity or version does not match its state row", id)
		}
		commander, err := domain.RestoreCommanderCell(record.Commander)
		if err != nil {
			return usecase.CellSet{}, fmt.Errorf("restore CommanderCell %q: %w", id, err)
		}
		set.Commanders = append(set.Commanders, commander)
		for _, stored := range record.Targets {
			target, err := domain.RestoreTargetCell(stored)
			if err != nil {
				return usecase.CellSet{}, fmt.Errorf("restore TargetCell %q: %w", stored.ID, err)
			}
			set.Targets = append(set.Targets, target)
		}
		for _, stored := range record.Dependencies {
			dependency, err := domain.RestoreDependencyCell(stored)
			if err != nil {
				return usecase.CellSet{}, fmt.Errorf("restore DependencyCell %q: %w", stored.ID, err)
			}
			set.Dependencies = append(set.Dependencies, dependency)
		}
	}
	if err := rows.Err(); err != nil {
		return usecase.CellSet{}, fmt.Errorf("iterate CommanderCells: %w", err)
	}
	if err := validateCellSet(set); err != nil {
		return usecase.CellSet{}, fmt.Errorf("validate persisted Cells: %w", err)
	}
	return set, nil
}

func validateCellSet(set usecase.CellSet) error {
	commanders := make(map[string]domain.CommanderCell, len(set.Commanders))
	groups := make(map[string]struct{}, len(set.Commanders))
	for _, commander := range set.Commanders {
		if _, exists := commanders[commander.ID]; exists {
			return fmt.Errorf("duplicate CommanderCell id %q", commander.ID)
		}
		commanders[commander.ID] = commander
		if commander.CellGroup == nil {
			return fmt.Errorf("CommanderCell %q has no CellGroup", commander.ID)
		}
		if _, exists := groups[commander.CellGroup.ID]; exists {
			return fmt.Errorf("duplicate CellGroup id %q", commander.CellGroup.ID)
		}
		groups[commander.CellGroup.ID] = struct{}{}
	}
	targets := make(map[string]domain.TargetCell, len(set.Targets))
	for _, target := range set.Targets {
		if _, exists := groups[target.CellGroupID]; !exists {
			return fmt.Errorf("TargetCell %q has unknown CellGroup %q", target.ID, target.CellGroupID)
		}
		if _, exists := targets[target.ID]; exists {
			return fmt.Errorf("duplicate TargetCell id %q", target.ID)
		}
		targets[target.ID] = target
	}
	dependencies := make(map[string]domain.DependencyCell, len(set.Dependencies))
	for _, dependency := range set.Dependencies {
		if _, exists := groups[dependency.CellGroupID]; !exists {
			return fmt.Errorf("DependencyCell %q has unknown CellGroup %q", dependency.ID, dependency.CellGroupID)
		}
		if _, exists := dependencies[dependency.ID]; exists {
			return fmt.Errorf("duplicate DependencyCell id %q", dependency.ID)
		}
		dependencies[dependency.ID] = dependency
	}
	return nil
}

type stateExecer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func applyChanges(ctx context.Context, execer stateExecer, current usecase.CellSet, next usecase.CellSet) error {
	currentRecords, err := cellRecords(current)
	if err != nil {
		return err
	}
	nextRecords, err := cellRecords(next)
	if err != nil {
		return err
	}
	currentByID := make(map[string]stateCellRecord, len(currentRecords))
	for _, record := range currentRecords {
		currentByID[record.Commander.ID] = record
	}
	nextByID := make(map[string]struct{}, len(nextRecords))
	for _, record := range nextRecords {
		nextByID[record.Commander.ID] = struct{}{}
	}
	// Release deleted positions before compacting the surviving groups.
	for _, record := range currentRecords {
		id := record.Commander.ID
		if _, exists := nextByID[id]; exists {
			continue
		}
		result, err := execer.ExecContext(ctx, "DELETE FROM cells WHERE id = ? AND version = ?", id, record.Commander.Version)
		if err != nil {
			return fmt.Errorf("delete CommanderCell %q: %w", id, err)
		}
		if err := requireOneRow(result, "delete", id, record.Commander.Version); err != nil {
			return err
		}
	}
	for position, record := range nextRecords {
		id := record.Commander.ID
		stored, exists := currentByID[id]
		if !exists {
			if record.Commander.Version != 1 {
				return fmt.Errorf("new CommanderCell %q must have version 1", id)
			}
			if err := insertCell(ctx, execer, position, record); err != nil {
				return err
			}
			continue
		}
		if reflect.DeepEqual(stored, record) && position == commanderPosition(current, id) {
			continue
		}
		if record.Commander.Version != stored.Commander.Version {
			return fmt.Errorf("%w: update CommanderCell %q expected version %d, found %d", domain.ErrVersionConflict, id, record.Commander.Version, stored.Commander.Version)
		}
		commander, err := domain.RestoreCommanderCell(record.Commander)
		if err != nil {
			return fmt.Errorf("restore updated CommanderCell %q: %w", id, err)
		}
		if err := commander.AdvanceVersion(); err != nil {
			return err
		}
		record.Commander = commander.Stored()
		if err := writeCell(ctx, execer, position, record, stored.Commander.Version); err != nil {
			return err
		}
	}
	return nil
}

func cellRecords(set usecase.CellSet) ([]stateCellRecord, error) {
	if err := validateCellSet(set); err != nil {
		return nil, err
	}
	records := make([]stateCellRecord, 0, len(set.Commanders))
	for _, commander := range set.Commanders {
		targets, dependencies := domain.SelectCellGroupMembersService(commander.CellGroup.ID, set.Targets, set.Dependencies)
		records = append(records, newStateCellRecord(commander, targets, dependencies))
	}
	return records, nil
}

func writeCell(ctx context.Context, execer stateExecer, position int, record stateCellRecord, expectedVersion uint64) error {
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encode CommanderCell %q: %w", record.Commander.ID, err)
	}
	result, err := execer.ExecContext(ctx, "UPDATE cells SET issue = ?, position = ?, version = ?, record = ? WHERE id = ? AND version = ?",
		record.Commander.CellGroup.Issue, position, record.Commander.Version, data, record.Commander.ID, expectedVersion)
	if err != nil {
		return fmt.Errorf("update CommanderCell %q: %w", record.Commander.ID, err)
	}
	return requireOneRow(result, "update", record.Commander.ID, expectedVersion)
}

func insertCell(ctx context.Context, execer stateExecer, position int, record stateCellRecord) error {
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encode CommanderCell %q: %w", record.Commander.ID, err)
	}
	if _, err := execer.ExecContext(ctx, "INSERT INTO cells (id, issue, position, version, record) VALUES (?, ?, ?, ?, ?)",
		record.Commander.ID, record.Commander.CellGroup.Issue, position, record.Commander.Version, data); err != nil {
		return fmt.Errorf("insert CommanderCell %q: %w", record.Commander.ID, err)
	}
	return nil
}

func requireOneRow(result sql.Result, operation string, id string, version uint64) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect %s CommanderCell %q: %w", operation, id, err)
	}
	if affected != 1 {
		return fmt.Errorf("%w: %s CommanderCell %q expected version %d", domain.ErrVersionConflict, operation, id, version)
	}
	return nil
}

func commanderPosition(set usecase.CellSet, id string) int {
	for position, commander := range set.Commanders {
		if commander.ID == id {
			return position
		}
	}
	return -1
}

func cloneCellSet(set usecase.CellSet) (usecase.CellSet, error) {
	cloned := usecase.NewCellSet(nil, nil, nil)
	for _, commander := range set.Commanders {
		cloned.Commanders = append(cloned.Commanders, commander.Clone())
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
