package domain

// SelectStartableQueueEntriesService returns the waiting entries, those no
// CellGroup refers to, that have no blocker, in Sequence order.
func SelectStartableQueueEntriesService(queue Queue, groups []CellGroup) []QueueEntry {
	var startable []QueueEntry
	for _, entry := range queue.Entries {
		if _, running := FindQueueEntryCellGroupService(groups, entry.ID); running {
			continue
		}
		if len(queue.Blockers(entry.ID)) == 0 {
			startable = append(startable, entry)
		}
	}
	sortQueueEntries(startable)
	return startable
}

// FindQueueEntryCellGroupService returns the CellGroup started from the entry.
// An entry without one is waiting.
func FindQueueEntryCellGroupService(groups []CellGroup, entryID string) (CellGroup, bool) {
	if entryID == "" {
		return CellGroup{}, false
	}
	for _, group := range groups {
		if group.QueueEntryID == entryID {
			return group, true
		}
	}
	return CellGroup{}, false
}
