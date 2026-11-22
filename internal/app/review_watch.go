package app

import (
	"sync"
	"time"

	"github.com/hgsg11/paracell/internal/query"
)

// reviewCheckInterval is how often the TUI asks the review provider about the
// Cells in review. The Cell list itself refreshes much more often.
const reviewCheckInterval = 45 * time.Second

// reviewWatch keeps the Cells that need a merged-or-abandoned decision, checked
// in the background so the TUI never waits for the provider.
type reviewWatch struct {
	check func() ([]string, error)
	now   func() time.Time

	mu        sync.Mutex
	undecided map[string]bool
	checked   time.Time
	running   bool
}

func newReviewWatch(check func() ([]string, error)) *reviewWatch {
	return &reviewWatch{check: check, now: time.Now}
}

// mark sets NeedsDecision from the latest check, and starts a new check when a
// Cell is in review and the last check is older than reviewCheckInterval.
func (w *reviewWatch) mark(cells []query.CellView) []query.CellView {
	w.mu.Lock()
	defer w.mu.Unlock()
	inReview := false
	for _, cell := range cells {
		inReview = inReview || len(cell.PullRequests) > 0
	}
	if inReview && !w.running && (w.checked.IsZero() || w.now().Sub(w.checked) >= reviewCheckInterval) {
		w.running = true
		go w.refresh()
	}
	for i := range cells {
		cells[i].NeedsDecision = w.undecided[cells[i].Name]
	}
	return cells
}

func (w *reviewWatch) refresh() {
	names, err := w.check()
	w.mu.Lock()
	defer w.mu.Unlock()
	w.running = false
	w.checked = w.now()
	// A failed check still reports the Cells it could read.
	_ = err
	w.undecided = make(map[string]bool, len(names))
	for _, name := range names {
		w.undecided[name] = true
	}
}
