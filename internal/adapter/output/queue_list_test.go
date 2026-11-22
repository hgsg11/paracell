package output

import (
	"testing"

	"github.com/hgsg11/paracell/internal/query"
)

func TestFormatWaitingQueueは待ちのエントリだけを出す(t *testing.T) {
	if got := FormatWaitingQueue([]query.QueueView{{Issue: "1", State: query.QueueStateRunning}}); got != "" {
		t.Fatalf("output = %q, want empty", got)
	}
	got := FormatWaitingQueue([]query.QueueView{
		{Issue: "1", State: query.QueueStateRunning},
		{Issue: "2", Note: "note", State: query.QueueStateWaiting, BlockedBy: []string{"1"}},
		{Issue: "3", State: query.QueueStateWaiting, BlockedBy: []string{"1", "2"}},
		{Issue: "4", State: query.QueueStateWaiting},
	})
	want := "WAITING\tNOTE\tBLOCKED_BY\n2\tnote\t1\n3\t-\t1,2\n4\t-\t-\n"
	if got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestFormatPlannedPathsはkindとpathとissueとnoteを出す(t *testing.T) {
	got := FormatPlannedPaths([]query.PlannedPathView{{Kind: "create", Path: "a.go", Issue: "1", Note: "n"}, {Kind: "modify", Path: "b.go", Issue: "2"}})
	want := "KIND\tPATH\tISSUE\tNOTE\ncreate\ta.go\t1\tn\nmodify\tb.go\t2\t-\n"
	if got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestFormatEnqueueResultは開始か待つ相手を出す(t *testing.T) {
	for _, c := range []struct {
		started   bool
		blockedBy []string
		want      string
	}{
		{true, nil, "started 3\n"},
		{false, []string{"1", "2"}, "waiting 3 for 1, 2\n"},
		{false, nil, "queued 3\n"},
	} {
		if got := FormatEnqueueResult("3", c.started, c.blockedBy); got != c.want {
			t.Fatalf("output = %q, want %q", got, c.want)
		}
	}
}
