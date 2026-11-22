package usecase

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/hgsg11/paracell/internal/domain"
)

// fakeFiles reports the listed workspace paths as present on disk.
type fakeFiles map[string]bool

func (f fakeFiles) ExistingPaths(_ context.Context, paths []domain.WorkspacePath) (domain.ExistingPaths, error) {
	var existing []domain.WorkspacePath
	for _, path := range paths {
		if f[path.Value] {
			existing = append(existing, path)
		}
	}
	return domain.NewExistingPaths(existing), nil
}

type queueTestPorts struct {
	*fakePorts
	ids   *sequenceIDGenerator
	files fakeFiles
}

func newQueueTestPorts(files ...string) queueTestPorts {
	ports := queueTestPorts{fakePorts: newFakePorts(), ids: &sequenceIDGenerator{}, files: fakeFiles{}}
	for _, file := range files {
		ports.files[file] = true
	}
	return ports
}

func (p queueTestPorts) fork() ForkCellUseCase {
	fork := newForkCellUseCase(p.fakePorts)
	fork.IDs = p.ids
	return fork
}

func (p queueTestPorts) enqueue(t *testing.T, input EnqueueCellInput) EnqueueCellOutput {
	t.Helper()
	output, err := p.enqueueUseCase().Execute(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	return output
}

func (p queueTestPorts) enqueueUseCase() EnqueueCellUseCase {
	return EnqueueCellUseCase{Config: p, Cells: p, Files: p.files, IDs: p.ids, Fork: p.fork()}
}

func (p queueTestPorts) groupForIssue(issue string) (domain.CellGroup, bool) {
	for _, group := range p.cells.Groups {
		if group.Issue == issue {
			return group, true
		}
	}
	return domain.CellGroup{}, false
}

func (p queueTestPorts) queuedIssues() []string {
	var issues []string
	for _, entry := range p.cells.Queue.Entries {
		issues = append(issues, entry.Issue)
	}
	return issues
}

func TestEnqueueCellはぶつからなければすぐにCellを作りQueueEntryIDを持たせる(t *testing.T) {
	ports := newQueueTestPorts("a.go")
	output := ports.enqueue(t, EnqueueCellInput{Issue: "1", Template: "feat", Modify: []string{"a.go"}, Create: []string{"b.go"}})
	if !output.Started || len(output.BlockedBy) != 0 {
		t.Fatalf("output = %#v", output)
	}
	group, ok := ports.groupForIssue("1")
	if !ok || group.QueueEntryID != output.Entry.ID || output.Entry.ID == "" || group.Creation.Status != domain.CreationReady {
		t.Fatalf("group = %#v, entry = %#v", group, output.Entry)
	}
	if len(ports.cells.Queue.Entries) != 1 {
		t.Fatalf("a running entry must stay in the queue: %#v", ports.cells.Queue)
	}
}

func TestEnqueueCellは重なる先行エントリがあれば待って相手を返す(t *testing.T) {
	ports := newQueueTestPorts("a.go")
	ports.enqueue(t, EnqueueCellInput{Issue: "1", Template: "feat", Modify: []string{"a.go"}})
	output := ports.enqueue(t, EnqueueCellInput{Issue: "2", Template: "feat", Modify: []string{"a.go"}})
	if output.Started || !reflect.DeepEqual(output.BlockedBy, []string{"1"}) {
		t.Fatalf("output = %#v", output)
	}
	if _, ok := ports.groupForIssue("2"); ok {
		t.Fatal("a waiting entry created a Cell")
	}
	disjoint := ports.enqueue(t, EnqueueCellInput{Issue: "3", Template: "feat", Create: []string{"c.go"}})
	if !disjoint.Started {
		t.Fatalf("disjoint entry did not start: %#v", disjoint)
	}
}

func TestEnqueueCellは形式の誤りとTemplateとPrefixをキューに入れる前に拒否する(t *testing.T) {
	cases := map[string]EnqueueCellInput{
		"absolute":         {Issue: "1", Template: "feat", Modify: []string{"/a.go"}},
		"outside":          {Issue: "1", Template: "feat", Modify: []string{"../a.go"}},
		"directory":        {Issue: "1", Template: "feat", Create: []string{"dir/"}},
		"glob":             {Issue: "1", Template: "feat", Create: []string{"*.go"}},
		"no paths":         {Issue: "1", Template: "feat"},
		"both kinds":       {Issue: "1", Template: "feat", Modify: []string{"a.go"}, Create: []string{"a.go"}},
		"unknown template": {Issue: "1", Template: "missing", Modify: []string{"a.go"}},
		"unknown prefix":   {Issue: "1", Template: "feat", Prefix: "missing", Modify: []string{"a.go"}},
		"modify missing":   {Issue: "1", Template: "feat", Modify: []string{"missing.go"}},
		"create existing":  {Issue: "1", Template: "feat", Create: []string{"a.go"}},
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			ports := newQueueTestPorts("a.go")
			if _, err := ports.enqueueUseCase().Execute(context.Background(), input); err == nil {
				t.Fatal("invalid input accepted")
			}
			if len(ports.cells.Queue.Entries) != 0 || len(ports.cells.Groups) != 0 {
				t.Fatalf("state changed: %#v", ports.cells)
			}
		})
	}
}

func TestEnqueueCellは同じissueのエントリやCellがあれば拒否する(t *testing.T) {
	ports := newQueueTestPorts("a.go")
	ports.cells = newUsecaseCellSet(t, [3]string{"cell-x", "9", "feat"})
	if _, err := ports.enqueueUseCase().Execute(context.Background(), EnqueueCellInput{Issue: "9", Template: "feat", Modify: []string{"a.go"}}); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("existing Cell: error = %v", err)
	}
	ports.enqueue(t, EnqueueCellInput{Issue: "1", Template: "feat", Modify: []string{"a.go"}})
	ports.enqueue(t, EnqueueCellInput{Issue: "2", Template: "feat", Modify: []string{"a.go"}})
	if _, err := ports.enqueueUseCase().Execute(context.Background(), EnqueueCellInput{Issue: "2", Template: "feat", Create: []string{"z.go"}}); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("existing entry: error = %v", err)
	}
}

func TestMarkCellMergedとAbandonedはエントリを消し待ちをSequence順に開始する(t *testing.T) {
	for _, mark := range []string{"merged", "abandon"} {
		t.Run(mark, func(t *testing.T) {
			ports := newQueueTestPorts("a.go", "b.go")
			ports.enqueue(t, EnqueueCellInput{Issue: "1", Template: "feat", Modify: []string{"a.go", "b.go"}})
			ports.enqueue(t, EnqueueCellInput{Issue: "2", Template: "feat", Modify: []string{"b.go"}})
			ports.enqueue(t, EnqueueCellInput{Issue: "3", Template: "feat", Modify: []string{"a.go"}})
			ports.enqueue(t, EnqueueCellInput{Issue: "4", Template: "feat", Modify: []string{"a.go"}})
			var err error
			if mark == "merged" {
				_, err = (MarkCellMergedUseCase{Cells: ports, Fork: ports.fork()}).Execute(context.Background(), MarkCellMergedInput{Cell: "1"})
			} else {
				_, err = (MarkCellAbandonedUseCase{Cells: ports, Fork: ports.fork()}).Execute(context.Background(), MarkCellAbandonedInput{Cell: "1"})
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := ports.queuedIssues(); !reflect.DeepEqual(got, []string{"2", "3", "4"}) {
				t.Fatalf("queue = %#v", got)
			}
			var started []string
			for _, group := range ports.cells.Groups {
				started = append(started, group.Issue)
			}
			// 4 still waits for 3, which started just now.
			if !reflect.DeepEqual(started, []string{"1", "2", "3"}) {
				t.Fatalf("cells = %#v", started)
			}
		})
	}
}

func TestMarkCellMergedはForkで作ったCellでも動きを変えない(t *testing.T) {
	ports := newQueueTestPorts("a.go")
	ports.cells = newUsecaseCellSet(t, [3]string{"cell-x", "9", "feat"})
	ports.enqueue(t, EnqueueCellInput{Issue: "1", Template: "feat", Modify: []string{"a.go"}})
	if _, err := (MarkCellMergedUseCase{Cells: ports, Fork: ports.fork()}).Execute(context.Background(), MarkCellMergedInput{Cell: "9"}); err != nil {
		t.Fatal(err)
	}
	if got := ports.queuedIssues(); !reflect.DeepEqual(got, []string{"1"}) {
		t.Fatalf("queue = %#v", got)
	}
}

func TestDequeueCellは待ちのエントリを消し実行中はエラーにする(t *testing.T) {
	ports := newQueueTestPorts("a.go")
	ports.enqueue(t, EnqueueCellInput{Issue: "1", Template: "feat", Modify: []string{"a.go"}})
	ports.enqueue(t, EnqueueCellInput{Issue: "2", Template: "feat", Modify: []string{"a.go"}})
	ports.enqueue(t, EnqueueCellInput{Issue: "3", Template: "feat", Modify: []string{"a.go"}})
	dequeue := DequeueCellUseCase{Cells: ports, Fork: ports.fork()}
	if _, err := dequeue.Execute(context.Background(), DequeueCellInput{Issue: "1"}); err == nil || !strings.Contains(err.Error(), "paracell merged") {
		t.Fatalf("running entry: error = %v", err)
	}
	if _, err := dequeue.Execute(context.Background(), DequeueCellInput{Issue: "missing"}); err == nil {
		t.Fatal("missing entry removed")
	}
	removed, err := dequeue.Execute(context.Background(), DequeueCellInput{Issue: "2"})
	if err != nil || removed.Issue != "2" {
		t.Fatalf("removed = %#v, err = %v", removed, err)
	}
	if got := ports.queuedIssues(); !reflect.DeepEqual(got, []string{"1", "3"}) {
		t.Fatalf("queue = %#v", got)
	}
	if _, ok := ports.groupForIssue("3"); ok {
		t.Fatal("3 started although 1 is still running")
	}
}

func TestQueueStarterはForkが失敗したエントリを実行中として残す(t *testing.T) {
	ports := newQueueTestPorts("a.go")
	ports.createSourceErr = errors.New("create source")
	_, err := ports.enqueueUseCase().Execute(context.Background(), EnqueueCellInput{Issue: "1", Template: "feat", Modify: []string{"a.go"}})
	if !errors.Is(err, ports.createSourceErr) {
		t.Fatalf("error = %v", err)
	}
	group, ok := ports.groupForIssue("1")
	if !ok || group.Creation.Status != domain.CreationFailed || group.QueueEntryID == "" {
		t.Fatalf("group = %#v", group)
	}
	ports.createSourceErr = nil
	output := ports.enqueue(t, EnqueueCellInput{Issue: "2", Template: "feat", Modify: []string{"a.go"}})
	if output.Started || !reflect.DeepEqual(output.BlockedBy, []string{"1"}) {
		t.Fatalf("output = %#v", output)
	}
}

func TestForkCellはキューの判定に入らない(t *testing.T) {
	ports := newQueueTestPorts("a.go")
	ports.enqueue(t, EnqueueCellInput{Issue: "1", Template: "feat", Modify: []string{"a.go"}})
	cell, err := ports.fork().Execute(context.Background(), ForkCellInput{Issue: "2", Template: "feat"})
	if err != nil {
		t.Fatal(err)
	}
	if group := ports.groupOf(t, cell); group.QueueEntryID != "" {
		t.Fatalf("fork set QueueEntryID: %#v", group)
	}
	if got := ports.queuedIssues(); !reflect.DeepEqual(got, []string{"1"}) {
		t.Fatalf("fork changed the queue: %#v", got)
	}
}
