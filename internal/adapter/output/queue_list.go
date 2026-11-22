package output

import (
	"fmt"
	"strings"

	"github.com/hgsg11/paracell/internal/query"
)

// FormatWaitingQueue lists the waiting entries and who they wait for. It
// returns an empty string when nothing waits; running entries are Cells and
// appear in the Cell list.
func FormatWaitingQueue(entries []query.QueueView) string {
	var b strings.Builder
	for _, entry := range entries {
		if entry.State != query.QueueStateWaiting {
			continue
		}
		if b.Len() == 0 {
			b.WriteString("WAITING\tNOTE\tBLOCKED_BY\n")
		}
		fmt.Fprintf(&b, "%s\t%s\t%s\n", entry.Issue, orDash(entry.Note), orDash(strings.Join(entry.BlockedBy, ",")))
	}
	return b.String()
}

// FormatPlannedPaths lists the paths declared by unfinished entries.
func FormatPlannedPaths(paths []query.PlannedPathView) string {
	var b strings.Builder
	b.WriteString("KIND\tPATH\tISSUE\tNOTE\n")
	for _, path := range paths {
		fmt.Fprintf(&b, "%s\t%s\t%s\t%s\n", path.Kind, path.Path, path.Issue, orDash(path.Note))
	}
	return b.String()
}

// FormatEnqueueResult tells whether the queued Cell started or whom it waits for.
func FormatEnqueueResult(issue string, started bool, blockedBy []string) string {
	if started {
		return fmt.Sprintf("started %s\n", issue)
	}
	if len(blockedBy) == 0 {
		return fmt.Sprintf("queued %s\n", issue)
	}
	return fmt.Sprintf("waiting %s for %s\n", issue, strings.Join(blockedBy, ", "))
}

func orDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}
