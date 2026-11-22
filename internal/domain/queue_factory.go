package domain

import "fmt"

// RestoredQueueEntry is the raw persisted values of one QueueEntry.
type RestoredQueueEntry struct {
	ID       string
	Issue    string
	Modify   []string
	Create   []string
	Template string
	Prefix   string
	Command  string
	Note     string
	Sequence uint64
}

// QueueFactory rebuilds the persisted Queue from raw values.
type QueueFactory struct{}

// Restore validates each entry's paths and fork request and checks that the IDs,
// issues, and sequences are unique. It does not check the paths against the
// disk: that happens only at enqueue.
func (QueueFactory) Restore(raw []RestoredQueueEntry) (Queue, error) {
	queue := NewQueue()
	ids, issues, sequences := map[string]bool{}, map[string]bool{}, map[uint64]bool{}
	for _, stored := range raw {
		if stored.ID == "" || stored.Issue == "" || stored.Sequence == 0 {
			return Queue{}, fmt.Errorf("restore queue entry %q: id, issue, and sequence are required", stored.ID)
		}
		if ids[stored.ID] || issues[stored.Issue] || sequences[stored.Sequence] {
			return Queue{}, fmt.Errorf("restore queue entry %q: duplicate id, issue, or sequence", stored.ID)
		}
		ids[stored.ID], issues[stored.Issue], sequences[stored.Sequence] = true, true, true
		modify, err := restoreWorkspacePaths(stored.Modify)
		if err != nil {
			return Queue{}, fmt.Errorf("restore queue entry %q: %w", stored.ID, err)
		}
		create, err := restoreWorkspacePaths(stored.Create)
		if err != nil {
			return Queue{}, fmt.Errorf("restore queue entry %q: %w", stored.ID, err)
		}
		plan, err := NewChangePlan(modify, create)
		if err != nil {
			return Queue{}, fmt.Errorf("restore queue entry %q: %w", stored.ID, err)
		}
		request, err := NewForkRequest(stored.Template, stored.Prefix, stored.Command, stored.Note)
		if err != nil {
			return Queue{}, fmt.Errorf("restore queue entry %q: %w", stored.ID, err)
		}
		queue.Entries = append(queue.Entries, QueueEntry{ID: stored.ID, Issue: stored.Issue, Plan: plan, Request: request, Sequence: stored.Sequence})
	}
	sortQueueEntries(queue.Entries)
	return queue, nil
}

func restoreWorkspacePaths(values []string) ([]WorkspacePath, error) {
	paths := make([]WorkspacePath, 0, len(values))
	for _, value := range values {
		p, err := NewWorkspacePath(value)
		if err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	return paths, nil
}
