package state

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hgsg11/paracell/internal/domain"
	_ "modernc.org/sqlite"
)

type SQLiteCellStateAdapter struct {
	Path string
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
	if _, err := conn.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
		return fmt.Errorf("begin state update: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.Background(), `ROLLBACK`)
		}
	}()
	cells, err := loadCells(ctx, conn)
	if err != nil {
		return err
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

func (a SQLiteCellStateAdapter) SaveCells(ctx context.Context, cells []domain.Cell) error {
	return a.UpdateCells(ctx, func([]domain.Cell) ([]domain.Cell, error) { return cells, nil })
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
	for _, statement := range []string{`PRAGMA busy_timeout = 10000`, `PRAGMA foreign_keys = ON`} {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			db.Close()
			return nil, fmt.Errorf("configure state database: %w", err)
		}
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
	return createSchema(ctx, db)
}

type stateExecer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func createSchema(ctx context.Context, execer stateExecer) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS cells (
			id TEXT PRIMARY KEY, issue TEXT NOT NULL, name TEXT NOT NULL, position INTEGER NOT NULL UNIQUE,
			note TEXT NOT NULL, template TEXT NOT NULL, base TEXT NOT NULL, branch TEXT NOT NULL,
			branch_mode TEXT NOT NULL, source_path TEXT NOT NULL, container_network TEXT NOT NULL,
			session_name TEXT NOT NULL, creation_status TEXT NOT NULL, creation_command TEXT NOT NULL,
			creation_failed_stage TEXT NOT NULL, creation_last_error TEXT NOT NULL,
			creation_attempt_id TEXT NOT NULL, creation_lease_started_at TEXT,
			creation_lease_heartbeat_at TEXT, status TEXT NOT NULL, done INTEGER NOT NULL
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS cells_issue_unique ON cells(issue) WHERE issue <> ''`,
		`CREATE UNIQUE INDEX IF NOT EXISTS cells_name_unique ON cells(name) WHERE name <> ''`,
		`CREATE TABLE IF NOT EXISTS cell_services (
			cell_id TEXT NOT NULL REFERENCES cells(id) ON DELETE CASCADE, role TEXT NOT NULL,
			container_name TEXT NOT NULL, source_container TEXT NOT NULL, volume_mode TEXT NOT NULL,
			database_present INTEGER NOT NULL, database_mode TEXT NOT NULL, database_system TEXT NOT NULL,
			database_copy_mode TEXT NOT NULL, PRIMARY KEY (cell_id, role)
		)`,
		`CREATE TABLE IF NOT EXISTS cell_sources (
			cell_id TEXT NOT NULL REFERENCES cells(id) ON DELETE CASCADE, position INTEGER NOT NULL,
			template_path TEXT NOT NULL, path TEXT NOT NULL, base TEXT NOT NULL, branch TEXT NOT NULL,
			PRIMARY KEY (cell_id, position)
		)`,
		`CREATE TABLE IF NOT EXISTS cell_session_windows (
			cell_id TEXT NOT NULL REFERENCES cells(id) ON DELETE CASCADE, position INTEGER NOT NULL,
			name TEXT NOT NULL, command TEXT NOT NULL, PRIMARY KEY (cell_id, position)
		)`,
		`CREATE TABLE IF NOT EXISTS cell_creation_stages (
			cell_id TEXT NOT NULL REFERENCES cells(id) ON DELETE CASCADE, position INTEGER NOT NULL,
			stage TEXT NOT NULL, PRIMARY KEY (cell_id, position)
		)`,
	}
	for _, statement := range statements {
		if _, err := execer.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("initialize state schema: %w", err)
		}
	}
	return nil
}

type stateQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func loadCells(ctx context.Context, queryer stateQueryer) ([]domain.Cell, error) {
	rows, err := queryer.QueryContext(ctx, `SELECT
		id, issue, name, note, template, base, branch, branch_mode, source_path,
		container_network, session_name, creation_status, creation_command,
		creation_failed_stage, creation_last_error, creation_attempt_id,
		creation_lease_started_at, creation_lease_heartbeat_at, status, done
		FROM cells ORDER BY position`)
	if err != nil {
		return nil, fmt.Errorf("query cells: %w", err)
	}
	cells := []domain.Cell{}
	for rows.Next() {
		var cell domain.Cell
		var legacyBase, legacyBranch, legacyBranchMode, legacySourcePath string
		var creationStatus, failedStage, status string
		var started, heartbeat sql.NullString
		var done int
		if err := rows.Scan(
			&cell.ID, &cell.Issue, &cell.Name, &cell.Note, &cell.Template, &legacyBase,
			&legacyBranch, &legacyBranchMode, &legacySourcePath, &cell.Containers.Network,
			&cell.Session.Name, &creationStatus, &cell.Creation.Command, &failedStage,
			&cell.Creation.LastError, &cell.Creation.AttemptID, &started, &heartbeat,
			&status, &done,
		); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan cell: %w", err)
		}
		_ = legacyBase
		_ = legacyBranch
		_ = legacyBranchMode
		_ = legacySourcePath
		cell.Creation.Status = domain.CreationStatus(creationStatus)
		cell.Creation.FailedStage = domain.CreationStage(failedStage)
		cell.Creation.LeaseStartedAt, err = parseTime(started)
		if err != nil {
			rows.Close()
			return nil, fmt.Errorf("decode cell %q lease start: %w", cell.ID, err)
		}
		cell.Creation.LeaseHeartbeatAt, err = parseTime(heartbeat)
		if err != nil {
			rows.Close()
			return nil, fmt.Errorf("decode cell %q lease heartbeat: %w", cell.ID, err)
		}
		if err := cell.SetStatus(domain.CellStatus(status)); err != nil {
			rows.Close()
			return nil, fmt.Errorf("decode cell %q status: %w", cell.ID, err)
		}
		if done != 0 {
			if err := cell.MarkDone(); err != nil {
				rows.Close()
				return nil, fmt.Errorf("decode cell %q done: %w", cell.ID, err)
			}
		}
		cells = append(cells, cell)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate cells: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close cells: %w", err)
	}
	for index := range cells {
		if err := loadCollections(ctx, queryer, &cells[index]); err != nil {
			return nil, err
		}
	}
	return cells, nil
}

func parseTime(value sql.NullString) (*time.Time, error) {
	if !value.Valid {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value.String)
	if err != nil {
		return nil, err
	}
	parsed = parsed.UTC()
	return &parsed, nil
}

func loadCollections(ctx context.Context, queryer stateQueryer, cell *domain.Cell) error {
	sourceRows, err := queryer.QueryContext(ctx, `SELECT template_path, path, base, branch
		FROM cell_sources WHERE cell_id = ? ORDER BY position`, cell.ID)
	if err != nil {
		return fmt.Errorf("query cell %q sources: %w", cell.ID, err)
	}
	var sources []domain.Source
	for sourceRows.Next() {
		var source domain.Source
		if err := sourceRows.Scan(&source.TemplatePath, &source.Path, &source.Base, &source.Branch); err != nil {
			sourceRows.Close()
			return fmt.Errorf("scan cell %q source: %w", cell.ID, err)
		}
		sources = append(sources, source)
	}
	if err := sourceRows.Close(); err != nil {
		return fmt.Errorf("close cell %q sources: %w", cell.ID, err)
	}
	if len(sources) != 0 {
		cell.Sources = sources
	}
	serviceRows, err := queryer.QueryContext(ctx, `SELECT role, container_name, source_container,
		volume_mode, database_present, database_mode, database_system, database_copy_mode
		FROM cell_services WHERE cell_id = ? ORDER BY role`, cell.ID)
	if err != nil {
		return fmt.Errorf("query cell %q services: %w", cell.ID, err)
	}
	for serviceRows.Next() {
		var role string
		var service domain.CellContainer
		var mode string
		var databasePresent int
		var ignoredMode, ignoredSystem, ignoredCopyMode string
		if err := serviceRows.Scan(&role, &service.ContainerName, &service.SourceContainer,
			&mode, &databasePresent, &ignoredMode, &ignoredSystem, &ignoredCopyMode); err != nil {
			serviceRows.Close()
			return fmt.Errorf("scan cell %q service: %w", cell.ID, err)
		}
		if cell.Containers.Services == nil {
			cell.Containers.Services = map[string]domain.CellContainer{}
		}
		_ = databasePresent
		_ = ignoredMode
		_ = ignoredSystem
		_ = ignoredCopyMode
		service.Mode, err = domain.NewMode(mode)
		if err != nil {
			service.Mode, _ = domain.NewMode(string(domain.Target))
		}
		cell.Containers.Services[role] = service
	}
	if err := serviceRows.Err(); err != nil {
		serviceRows.Close()
		return fmt.Errorf("iterate cell %q services: %w", cell.ID, err)
	}
	if err := serviceRows.Close(); err != nil {
		return fmt.Errorf("close cell %q services: %w", cell.ID, err)
	}
	windowRows, err := queryer.QueryContext(ctx, `SELECT name, command FROM cell_session_windows
		WHERE cell_id = ? ORDER BY position`, cell.ID)
	if err != nil {
		return fmt.Errorf("query cell %q session windows: %w", cell.ID, err)
	}
	for windowRows.Next() {
		var window domain.SessionWindow
		if err := windowRows.Scan(&window.Name, &window.Command); err != nil {
			windowRows.Close()
			return fmt.Errorf("scan cell %q session window: %w", cell.ID, err)
		}
		cell.Session.Windows = append(cell.Session.Windows, window)
	}
	if err := windowRows.Err(); err != nil {
		windowRows.Close()
		return fmt.Errorf("iterate cell %q session windows: %w", cell.ID, err)
	}
	if err := windowRows.Close(); err != nil {
		return fmt.Errorf("close cell %q session windows: %w", cell.ID, err)
	}
	stageRows, err := queryer.QueryContext(ctx, `SELECT stage FROM cell_creation_stages
		WHERE cell_id = ? ORDER BY position`, cell.ID)
	if err != nil {
		return fmt.Errorf("query cell %q creation stages: %w", cell.ID, err)
	}
	for stageRows.Next() {
		var stage string
		if err := stageRows.Scan(&stage); err != nil {
			stageRows.Close()
			return fmt.Errorf("scan cell %q creation stage: %w", cell.ID, err)
		}
		cell.Creation.CompletedStages = append(cell.Creation.CompletedStages, domain.CreationStage(stage))
	}
	if err := stageRows.Err(); err != nil {
		stageRows.Close()
		return fmt.Errorf("iterate cell %q creation stages: %w", cell.ID, err)
	}
	if err := stageRows.Close(); err != nil {
		return fmt.Errorf("close cell %q creation stages: %w", cell.ID, err)
	}
	return nil
}

func replaceCells(ctx context.Context, execer stateExecer, cells []domain.Cell) error {
	if _, err := execer.ExecContext(ctx, `DELETE FROM cells`); err != nil {
		return fmt.Errorf("clear cells: %w", err)
	}
	for position, cell := range cells {
		if err := insertCell(ctx, execer, position, cell); err != nil {
			return err
		}
	}
	return nil
}

func insertCell(ctx context.Context, execer stateExecer, position int, cell domain.Cell) error {
	var primary domain.Source
	if len(cell.Sources) != 0 {
		primary = cell.Sources[0]
	}
	if _, err := execer.ExecContext(ctx, `INSERT INTO cells (
		id, issue, name, position, note, template, base, branch, branch_mode, source_path,
		container_network, session_name, creation_status, creation_command,
		creation_failed_stage, creation_last_error, creation_attempt_id,
		creation_lease_started_at, creation_lease_heartbeat_at, status, done
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		cell.ID, cell.Issue, cell.Name, position, cell.Note, cell.Template, primary.Base,
		primary.Branch, "", primary.Path, cell.Containers.Network,
		cell.Session.Name, cell.Creation.Status, cell.Creation.Command,
		cell.Creation.FailedStage, cell.Creation.LastError, cell.Creation.AttemptID,
		storedTime(cell.Creation.LeaseStartedAt), storedTime(cell.Creation.LeaseHeartbeatAt),
		cell.Status(), cell.IsDone(),
	); err != nil {
		return fmt.Errorf("insert cell %q: %w", cell.ID, err)
	}
	for sourcePosition, source := range cell.Sources {
		if _, err := execer.ExecContext(ctx, `INSERT INTO cell_sources
			(cell_id, position, template_path, path, base, branch) VALUES (?, ?, ?, ?, ?, ?)`,
			cell.ID, sourcePosition, source.TemplatePath, source.Path, source.Base, source.Branch); err != nil {
			return fmt.Errorf("insert cell %q source: %w", cell.ID, err)
		}
	}
	for role, service := range cell.Containers.Services {
		if _, err := execer.ExecContext(ctx, `INSERT INTO cell_services (
			cell_id, role, container_name, source_container, volume_mode, database_present,
			database_mode, database_system, database_copy_mode
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, cell.ID, role, service.ContainerName,
			service.SourceContainer, string(service.Mode), false, "", "", "",
		); err != nil {
			return fmt.Errorf("insert cell %q service %q: %w", cell.ID, role, err)
		}
	}
	for windowPosition, window := range cell.Session.Windows {
		if _, err := execer.ExecContext(ctx, `INSERT INTO cell_session_windows
			(cell_id, position, name, command) VALUES (?, ?, ?, ?)`,
			cell.ID, windowPosition, window.Name, window.Command); err != nil {
			return fmt.Errorf("insert cell %q session window: %w", cell.ID, err)
		}
	}
	for stagePosition, stage := range cell.Creation.CompletedStages {
		if _, err := execer.ExecContext(ctx, `INSERT INTO cell_creation_stages
			(cell_id, position, stage) VALUES (?, ?, ?)`, cell.ID, stagePosition, stage); err != nil {
			return fmt.Errorf("insert cell %q creation stage: %w", cell.ID, err)
		}
	}
	return nil
}

func storedTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC().Format(time.RFC3339Nano)
}
