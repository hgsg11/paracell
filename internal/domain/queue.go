package domain

import (
	"fmt"
	"sort"
	"strings"
)

// QueueEntry is a Cell waiting in or started from the Queue. It does not know
// its Cell: a CellGroup started from it refers to it by QueueEntryID. Sequence
// is the enqueue order.
type QueueEntry struct {
	ID       string
	Issue    string
	Plan     ChangePlan
	Request  ForkRequest
	Sequence uint64
}

// Queue is the aggregate root, one per project, that holds the unfinished
// entries. Its rules span entries: an issue appears once, and a created path is
// declared by at most one entry. An entry is removed once its Cell is merged or
// abandoned, so every entry in the Queue is unfinished.
type Queue struct {
	Entries []QueueEntry
}

func NewQueue() Queue {
	return Queue{}
}

// Enqueue validates the entry against the unfinished entries and the paths that
// exist on disk, then appends it with the next Sequence. A modify path must
// exist on disk or be created by another entry; a create path must exist in
// neither. Every violation is reported in one error.
func (q *Queue) Enqueue(id, issue string, plan ChangePlan, request ForkRequest, existing ExistingPaths) (QueueEntry, error) {
	if id == "" || issue == "" {
		return QueueEntry{}, fmt.Errorf("queue entry id and issue are required")
	}
	if len(plan.Modify) == 0 && len(plan.Create) == 0 {
		return QueueEntry{}, fmt.Errorf("queue entry %q requires a change plan", issue)
	}
	if request.Template == "" {
		return QueueEntry{}, fmt.Errorf("queue entry %q requires a fork request", issue)
	}
	name := NewCellName(issue)
	var sequence uint64
	creators := map[WorkspacePath]string{}
	for _, entry := range q.Entries {
		if entry.ID == id {
			return QueueEntry{}, fmt.Errorf("queue entry id %q already exists", id)
		}
		if entry.Issue == issue {
			return QueueEntry{}, fmt.Errorf("queue entry for issue %q already exists", issue)
		}
		if NewCellName(entry.Issue) == name {
			return QueueEntry{}, fmt.Errorf("queue entry with cell name %q already exists", name.Value)
		}
		sequence = max(sequence, entry.Sequence)
		for _, p := range entry.Plan.Create {
			creators[p] = entry.Issue
		}
	}
	var violations []string
	for _, p := range plan.Modify {
		if _, created := creators[p]; !existing.Contains(p) && !created {
			violations = append(violations, fmt.Sprintf("modify %q: does not exist on disk and no queued entry creates it", p.Value))
		}
	}
	for _, p := range plan.Create {
		if existing.Contains(p) {
			violations = append(violations, fmt.Sprintf("create %q: already exists on disk", p.Value))
		}
		if creator, created := creators[p]; created {
			violations = append(violations, fmt.Sprintf("create %q: already declared as create by %s", p.Value, creator))
		}
	}
	if len(violations) > 0 {
		return QueueEntry{}, fmt.Errorf("queue entry %q has invalid paths:\n  %s", issue, strings.Join(violations, "\n  "))
	}
	entry := QueueEntry{ID: id, Issue: issue, Plan: plan, Request: request, Sequence: sequence + 1}
	q.Entries = append(append([]QueueEntry(nil), q.Entries...), entry)
	return entry, nil
}

// Remove deletes the entry. It fails with ErrNotFound when the entry is not queued.
func (q *Queue) Remove(entryID string) error {
	for i, entry := range q.Entries {
		if entry.ID == entryID {
			q.Entries = append(append([]QueueEntry(nil), q.Entries[:i]...), q.Entries[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("queue entry %q: %w", entryID, ErrNotFound)
}

// Entry returns the entry with the given ID.
func (q Queue) Entry(entryID string) (QueueEntry, bool) {
	for _, entry := range q.Entries {
		if entry.ID == entryID {
			return entry, true
		}
	}
	return QueueEntry{}, false
}

// EntryForIssue returns the entry queued for the issue.
func (q Queue) EntryForIssue(issue string) (QueueEntry, bool) {
	for _, entry := range q.Entries {
		if entry.Issue == issue {
			return entry, true
		}
	}
	return QueueEntry{}, false
}

// Blockers returns the entries, waiting or running, enqueued before the entry
// whose plans overlap it, in Sequence order.
func (q Queue) Blockers(entryID string) []QueueEntry {
	target, ok := q.Entry(entryID)
	if !ok {
		return nil
	}
	var blockers []QueueEntry
	for _, entry := range q.Entries {
		if entry.Sequence < target.Sequence && entry.Plan.Overlaps(target.Plan) {
			blockers = append(blockers, entry)
		}
	}
	sortQueueEntries(blockers)
	return blockers
}

func sortQueueEntries(entries []QueueEntry) {
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Sequence < entries[j].Sequence })
}
