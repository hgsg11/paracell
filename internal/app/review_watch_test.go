package app

import (
	"testing"
	"time"

	"github.com/hgsg11/paracell/internal/query"
)

func TestReviewWatchはレビュー中のCellがあるときだけ間隔を空けて確かめる(t *testing.T) {
	checks := make(chan struct{}, 4)
	now := time.Date(2026, 11, 23, 10, 0, 0, 0, time.UTC)
	watch := newReviewWatch(func() ([]string, error) {
		checks <- struct{}{}
		return []string{"456"}, nil
	})
	watch.now = func() time.Time { return now }

	watch.mark([]query.CellView{{Name: "123"}})
	select {
	case <-checks:
		t.Fatal("checked without a Cell in review")
	case <-time.After(20 * time.Millisecond):
	}
	cells := []query.CellView{{Name: "123"}, {Name: "456", PullRequests: []string{"1"}}}
	watch.mark(cells)
	<-checks
	waitFor(t, func() bool { return watch.mark(cells)[1].NeedsDecision })
	if cells[0].NeedsDecision {
		t.Fatal("cell 123 marked")
	}
	select {
	case <-checks:
		t.Fatal("checked again before the interval")
	case <-time.After(20 * time.Millisecond):
	}
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met")
		}
		time.Sleep(time.Millisecond)
	}
}
