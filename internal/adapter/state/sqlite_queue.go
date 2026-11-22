package state

import (
	"context"
	"fmt"
	"reflect"

	"github.com/hgsg11/paracell/internal/domain"
)

const (
	pathKindModify = "modify"
	pathKindCreate = "create"
)

// queueSchema stores one row per QueueEntry and one row per declared path. The
// {kind, path} rows are the adapter's shape; the domain keeps the two lists.
var queueSchema = []string{
	`CREATE TABLE IF NOT EXISTS queue_entries (
		id TEXT PRIMARY KEY,
		issue TEXT NOT NULL UNIQUE,
		sequence INTEGER NOT NULL UNIQUE,
		template TEXT NOT NULL,
		prefix TEXT NOT NULL,
		command TEXT NOT NULL,
		note TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS queue_entry_paths (
		entry_id TEXT NOT NULL REFERENCES queue_entries(id) ON DELETE CASCADE,
		position INTEGER NOT NULL,
		kind TEXT NOT NULL CHECK (kind IN ('modify', 'create')),
		path TEXT NOT NULL,
		PRIMARY KEY (entry_id, position),
		UNIQUE (entry_id, path)
	)`,
}

// loadQueue reads every QueueEntry in Sequence order and restores the Queue.
func loadQueue(ctx context.Context, queryer stateQueryer) (domain.Queue, error) {
	rows, err := queryer.QueryContext(ctx, "SELECT id, issue, sequence, template, prefix, command, note FROM queue_entries ORDER BY sequence")
	if err != nil {
		return domain.Queue{}, fmt.Errorf("query queue entries: %w", err)
	}
	var raw []domain.RestoredQueueEntry
	index := map[string]int{}
	for rows.Next() {
		var entry domain.RestoredQueueEntry
		if err := rows.Scan(&entry.ID, &entry.Issue, &entry.Sequence, &entry.Template, &entry.Prefix, &entry.Command, &entry.Note); err != nil {
			rows.Close()
			return domain.Queue{}, fmt.Errorf("scan queue entry: %w", err)
		}
		index[entry.ID] = len(raw)
		raw = append(raw, entry)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return domain.Queue{}, fmt.Errorf("iterate queue entries: %w", err)
	}
	rows.Close()
	paths, err := queryer.QueryContext(ctx, "SELECT entry_id, kind, path FROM queue_entry_paths ORDER BY entry_id, position")
	if err != nil {
		return domain.Queue{}, fmt.Errorf("query queue entry paths: %w", err)
	}
	defer paths.Close()
	for paths.Next() {
		var entryID, kind, path string
		if err := paths.Scan(&entryID, &kind, &path); err != nil {
			return domain.Queue{}, fmt.Errorf("scan queue entry path: %w", err)
		}
		i, ok := index[entryID]
		if !ok {
			return domain.Queue{}, fmt.Errorf("queue entry path %q refers to unknown entry %q", path, entryID)
		}
		switch kind {
		case pathKindModify:
			raw[i].Modify = append(raw[i].Modify, path)
		case pathKindCreate:
			raw[i].Create = append(raw[i].Create, path)
		default:
			return domain.Queue{}, fmt.Errorf("queue entry %q has unsupported path kind %q", entryID, kind)
		}
	}
	if err := paths.Err(); err != nil {
		return domain.Queue{}, fmt.Errorf("iterate queue entry paths: %w", err)
	}
	return domain.QueueFactory{}.Restore(raw)
}

// applyQueueChanges deletes the entries that left the Queue and inserts the new
// ones. An entry never changes after it is enqueued.
func applyQueueChanges(ctx context.Context, execer stateExecer, current domain.Queue, next domain.Queue) error {
	currentByID := make(map[string]domain.QueueEntry, len(current.Entries))
	for _, entry := range current.Entries {
		currentByID[entry.ID] = entry
	}
	nextByID := make(map[string]struct{}, len(next.Entries))
	for _, entry := range next.Entries {
		if _, exists := nextByID[entry.ID]; exists {
			return fmt.Errorf("duplicate queue entry id %q", entry.ID)
		}
		nextByID[entry.ID] = struct{}{}
	}
	for _, entry := range current.Entries {
		if _, exists := nextByID[entry.ID]; exists {
			continue
		}
		if _, err := execer.ExecContext(ctx, "DELETE FROM queue_entries WHERE id = ?", entry.ID); err != nil {
			return fmt.Errorf("delete queue entry %q: %w", entry.ID, err)
		}
	}
	for _, entry := range next.Entries {
		stored, exists := currentByID[entry.ID]
		if exists {
			if !reflect.DeepEqual(stored, entry) {
				return fmt.Errorf("queue entry %q cannot change after it is enqueued", entry.ID)
			}
			continue
		}
		if err := insertQueueEntry(ctx, execer, entry); err != nil {
			return err
		}
	}
	return nil
}

func insertQueueEntry(ctx context.Context, execer stateExecer, entry domain.QueueEntry) error {
	request := entry.Request
	if _, err := execer.ExecContext(ctx, "INSERT INTO queue_entries (id, issue, sequence, template, prefix, command, note) VALUES (?, ?, ?, ?, ?, ?, ?)",
		entry.ID, entry.Issue, entry.Sequence, request.Template, request.Prefix, request.Command, request.Note); err != nil {
		return fmt.Errorf("insert queue entry %q: %w", entry.Issue, err)
	}
	position := 0
	for _, group := range []struct {
		kind  string
		paths []domain.WorkspacePath
	}{{pathKindModify, entry.Plan.Modify}, {pathKindCreate, entry.Plan.Create}} {
		for _, path := range group.paths {
			if _, err := execer.ExecContext(ctx, "INSERT INTO queue_entry_paths (entry_id, position, kind, path) VALUES (?, ?, ?, ?)",
				entry.ID, position, group.kind, path.Value); err != nil {
				return fmt.Errorf("insert queue entry %q path %q: %w", entry.Issue, path.Value, err)
			}
			position++
		}
	}
	return nil
}

// cloneQueue copies the entries and their path lists so that the update
// callback cannot change the loaded Queue.
func cloneQueue(queue domain.Queue) domain.Queue {
	cloned := domain.NewQueue()
	for _, entry := range queue.Entries {
		entry.Plan = domain.ChangePlan{
			Modify: append([]domain.WorkspacePath(nil), entry.Plan.Modify...),
			Create: append([]domain.WorkspacePath(nil), entry.Plan.Create...),
		}
		cloned.Entries = append(cloned.Entries, entry)
	}
	return cloned
}
