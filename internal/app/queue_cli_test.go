package app

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hgsg11/paracell/internal/adapter/state"
	"github.com/hgsg11/paracell/internal/domain"
	"github.com/hgsg11/paracell/internal/usecase"
)

func TestQueueコマンドはmodifyとcreateを繰り返し解析できる(t *testing.T) {
	cmd, err := ParseCommand([]string{"queue", "173", "--modify", "a.go", "--template", "feat", "--create", "b.go", "--modify", "c.go", "--note", "n", "--prefix", "fix", "--command", "run"})
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Kind != CommandQueue || cmd.Issue != "173" || cmd.Template != "feat" || cmd.Prefix != "fix" || cmd.Command != "run" || cmd.Note == nil || *cmd.Note != "n" ||
		!reflect.DeepEqual(cmd.Modify, []string{"a.go", "c.go"}) || !reflect.DeepEqual(cmd.Create, []string{"b.go"}) {
		t.Fatalf("cmd = %#v", cmd)
	}
	for _, args := range [][]string{
		{"queue", "173", "--template", "feat"},
		{"queue", "173", "--modify", "a.go"},
		{"queue", "173", "--template", "feat", "--template", "fix", "--modify", "a.go"},
		{"queue", "173", "--template", "feat", "--unknown", "x", "--modify", "a.go"},
		{"queue", "173", "--template", "feat", "--modify"},
		{"queue", "", "--template", "feat", "--modify", "a.go"},
	} {
		if _, err := ParseCommand(args); err == nil {
			t.Fatalf("ParseCommand(%v) accepted", args)
		}
	}
}

func TestPlannedとDequeueコマンドを解析できる(t *testing.T) {
	planned, err := ParseCommand([]string{"planned"})
	if err != nil || planned.Kind != CommandPlanned || planned.PathPrefix != "" {
		t.Fatalf("planned = %#v, %v", planned, err)
	}
	prefixed, err := ParseCommand([]string{"planned", "internal/"})
	if err != nil || prefixed.PathPrefix != "internal/" {
		t.Fatalf("prefixed = %#v, %v", prefixed, err)
	}
	dequeue, err := ParseCommand([]string{"dequeue", "173"})
	if err != nil || dequeue.Kind != CommandDequeue || dequeue.Issue != "173" {
		t.Fatalf("dequeue = %#v, %v", dequeue, err)
	}
	for _, args := range [][]string{{"planned", "a", "b"}, {"dequeue"}, {"dequeue", ""}, {"dequeue", "1", "2"}} {
		if _, err := ParseCommand(args); err == nil {
			t.Fatalf("ParseCommand(%v) accepted", args)
		}
	}
}

// TestRunはQueueで待ちを作りPlannedとLsで見せDequeueで消す runs the commands
// that do not fork: the first entry is already running as a Cell.
func TestRunはQueueで待ちを作りPlannedとLsで見せDequeueで消す(t *testing.T) {
	t.Setenv("PARACELL_ROOT", "")
	dir := t.TempDir()
	config := []byte("project:\n  name: myapp\nproviders:\n  source: git\n  workspace: tmux\ntemplates:\n  default:\n    commanderCell:\n      name: workspace\n      workspace:\n        windows: []\n")
	if err := os.WriteFile(filepath.Join(dir, "paracell.yaml"), config, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.go"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	store := state.SQLiteCellAdapter{Path: filepath.Join(dir, ".paracell", "state.db")}
	running := appTestCell("cell-1", "1", "default")
	running.Group.QueueEntryID = "e1"
	if err := store.UpdateCells(context.Background(), func(set usecase.CellSet) (usecase.CellSet, error) {
		path, _ := domain.NewWorkspacePath("a.go")
		plan, _ := domain.NewChangePlan([]domain.WorkspacePath{path}, nil)
		if _, err := set.Queue.Enqueue("e1", "1", plan, domain.ForkRequest{Template: "default"}, domain.NewExistingPaths([]domain.WorkspacePath{path})); err != nil {
			return set, err
		}
		set.Groups = append(set.Groups, running.Group)
		set.Commanders = append(set.Commanders, running.Commander)
		return set, nil
	}); err != nil {
		t.Fatal(err)
	}

	output, err := captureStdout(func() error {
		return Run(context.Background(), []string{"queue", "2", "--template", "default", "--modify", "a.go", "--create", "b.go", "--note", "second"}, dir)
	})
	if err != nil || output != "waiting 2 for 1\n" {
		t.Fatalf("queue output = %q, err = %v", output, err)
	}
	if _, err := captureStdout(func() error {
		return Run(context.Background(), []string{"queue", "3", "--template", "default", "--modify", "missing.go", "--create", "a.go"}, dir)
	}); err == nil || !strings.Contains(err.Error(), `modify "missing.go"`) || !strings.Contains(err.Error(), `create "a.go"`) {
		t.Fatalf("invalid queue error = %v", err)
	}

	output, err = captureStdout(func() error { return Run(context.Background(), []string{"planned", "b"}, dir) })
	if err != nil || output != "KIND\tPATH\tISSUE\tNOTE\ncreate\tb.go\t2\tsecond\n" {
		t.Fatalf("planned output = %q, err = %v", output, err)
	}
	output, err = captureStdout(func() error { return Run(context.Background(), []string{"ls"}, dir) })
	if err != nil || !strings.HasSuffix(output, "\nWAITING\tNOTE\tBLOCKED_BY\n2\tsecond\t1\n") {
		t.Fatalf("ls output = %q, err = %v", output, err)
	}

	if err := Run(context.Background(), []string{"dequeue", "1"}, dir); err == nil || !strings.Contains(err.Error(), "paracell merged") {
		t.Fatalf("dequeue running error = %v", err)
	}
	if err := Run(context.Background(), []string{"dequeue", "2"}, dir); err != nil {
		t.Fatal(err)
	}
	set, err := store.LoadCells(context.Background())
	if err != nil || len(set.Queue.Entries) != 1 || set.Queue.Entries[0].Issue != "1" {
		t.Fatalf("queue = %#v, %v", set.Queue, err)
	}
}
