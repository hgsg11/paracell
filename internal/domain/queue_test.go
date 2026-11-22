package domain

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

var testForkRequest = ForkRequest{Template: "feat"}

func enqueueTest(t *testing.T, queue *Queue, id, issue string, modify []string, create []string, existing ...string) QueueEntry {
	t.Helper()
	entry, err := queue.Enqueue(id, issue, planOf(t, modify, create), testForkRequest, existingOf(t, existing...))
	if err != nil {
		t.Fatal(err)
	}
	return entry
}

func planOf(t *testing.T, modify []string, create []string) ChangePlan {
	t.Helper()
	var modifyPaths, createPaths []WorkspacePath
	for _, value := range modify {
		modifyPaths = append(modifyPaths, mustPath(t, value))
	}
	for _, value := range create {
		createPaths = append(createPaths, mustPath(t, value))
	}
	return mustPlan(t, modifyPaths, createPaths)
}

func existingOf(t *testing.T, values ...string) ExistingPaths {
	t.Helper()
	var paths []WorkspacePath
	for _, value := range values {
		paths = append(paths, mustPath(t, value))
	}
	return NewExistingPaths(paths)
}

func issuesOf(entries []QueueEntry) []string {
	var issues []string
	for _, entry := range entries {
		issues = append(issues, entry.Issue)
	}
	return issues
}

func TestQueueEnqueueは順番を採番してエントリを足す(t *testing.T) {
	queue := NewQueue()
	first := enqueueTest(t, &queue, "e1", "1", []string{"a.go"}, nil, "a.go")
	second := enqueueTest(t, &queue, "e2", "2", nil, []string{"b.go"})
	if first.Sequence != 1 || second.Sequence != 2 || len(queue.Entries) != 2 {
		t.Fatalf("entries = %#v", queue.Entries)
	}
	if second.Request != testForkRequest || !reflect.DeepEqual(second.Plan, planOf(t, nil, []string{"b.go"})) {
		t.Fatalf("second = %#v", second)
	}
	if err := queue.Remove("e2"); err != nil {
		t.Fatal(err)
	}
	third := enqueueTest(t, &queue, "e3", "3", nil, []string{"c.go"})
	if third.Sequence != 2 {
		t.Fatalf("sequence after remove = %d, want the next after the remaining entries", third.Sequence)
	}
}

func TestQueueEnqueueのmodifyはディスクか未完了のcreateにあれば通る(t *testing.T) {
	queue := NewQueue()
	enqueueTest(t, &queue, "e1", "1", nil, []string{"new.go"})
	enqueueTest(t, &queue, "e2", "2", []string{"new.go", "disk.go"}, nil, "disk.go")
	if _, err := queue.Enqueue("e3", "3", planOf(t, []string{"missing.go"}, nil), testForkRequest, existingOf(t)); err == nil {
		t.Fatal("modify of a missing path accepted")
	}
}

func TestQueueEnqueueのcreateはディスクにも未完了のcreateにもなければ通る(t *testing.T) {
	queue := NewQueue()
	enqueueTest(t, &queue, "e1", "1", nil, []string{"new.go"})
	if _, err := queue.Enqueue("e2", "2", planOf(t, nil, []string{"disk.go"}), testForkRequest, existingOf(t, "disk.go")); err == nil {
		t.Fatal("create of a path on disk accepted")
	}
	if _, err := queue.Enqueue("e3", "3", planOf(t, nil, []string{"new.go"}), testForkRequest, existingOf(t)); err == nil {
		t.Fatal("create of a path another entry creates accepted")
	}
}

func TestQueueEnqueueは違反をまとめて1つのエラーで返す(t *testing.T) {
	queue := NewQueue()
	enqueueTest(t, &queue, "e1", "1", nil, []string{"queued.go"})
	_, err := queue.Enqueue("e2", "2", planOf(t, []string{"missing.go"}, []string{"disk.go", "queued.go"}), testForkRequest, existingOf(t, "disk.go"))
	if err == nil {
		t.Fatal("invalid entry accepted")
	}
	for _, want := range []string{`modify "missing.go"`, `create "disk.go"`, `create "queued.go"`, "by 1"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not mention %q", err, want)
		}
	}
	if len(queue.Entries) != 1 {
		t.Fatalf("rejected entry was added: %#v", queue.Entries)
	}
}

func TestQueueEnqueueは同じissueとIDを拒否する(t *testing.T) {
	queue := NewQueue()
	enqueueTest(t, &queue, "e1", "1", nil, []string{"a.go"})
	if _, err := queue.Enqueue("e2", "1", planOf(t, nil, []string{"b.go"}), testForkRequest, existingOf(t)); err == nil {
		t.Fatal("same issue accepted")
	}
	if _, err := queue.Enqueue("e1", "2", planOf(t, nil, []string{"b.go"}), testForkRequest, existingOf(t)); err == nil {
		t.Fatal("same id accepted")
	}
	if _, err := queue.Enqueue("e3", "", planOf(t, nil, []string{"b.go"}), testForkRequest, existingOf(t)); err == nil {
		t.Fatal("empty issue accepted")
	}
	if _, err := queue.Enqueue("e3", "3", planOf(t, nil, []string{"b.go"}), ForkRequest{}, existingOf(t)); err == nil {
		t.Fatal("empty fork request accepted")
	}
}

func TestQueueRemoveは見つからないエントリをErrNotFoundにする(t *testing.T) {
	queue := NewQueue()
	enqueueTest(t, &queue, "e1", "1", nil, []string{"a.go"})
	if err := queue.Remove("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if err := queue.Remove("e1"); err != nil || len(queue.Entries) != 0 {
		t.Fatalf("err = %v, entries = %#v", err, queue.Entries)
	}
}

func TestQueueEnqueueとRemoveは元のスライスを書き換えない(t *testing.T) {
	queue := NewQueue()
	enqueueTest(t, &queue, "e1", "1", nil, []string{"a.go"})
	enqueueTest(t, &queue, "e2", "2", nil, []string{"b.go"})
	snapshot := queue
	if err := queue.Remove("e1"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(issuesOf(snapshot.Entries), []string{"1", "2"}) {
		t.Fatalf("snapshot changed: %#v", snapshot.Entries)
	}
}

func TestQueueBlockersは先に入った重なるエントリをSequence順に返す(t *testing.T) {
	queue := NewQueue()
	enqueueTest(t, &queue, "e1", "1", nil, []string{"a.go"})
	enqueueTest(t, &queue, "e2", "2", []string{"x.go"}, nil, "x.go")
	enqueueTest(t, &queue, "e3", "3", []string{"a.go", "x.go"}, nil, "x.go")
	enqueueTest(t, &queue, "e4", "4", []string{"y.go"}, nil, "y.go")
	if got := issuesOf(queue.Blockers("e3")); !reflect.DeepEqual(got, []string{"1", "2"}) {
		t.Fatalf("blockers of 3 = %#v", got)
	}
	if got := queue.Blockers("e1"); len(got) != 0 {
		t.Fatalf("first entry has blockers: %#v", got)
	}
	if got := queue.Blockers("e4"); len(got) != 0 {
		t.Fatalf("disjoint entry has blockers: %#v", got)
	}
	if got := queue.Blockers("missing"); got != nil {
		t.Fatalf("missing entry has blockers: %#v", got)
	}
}

func TestSelectStartableQueueEntriesServiceは待ちでBlockerのないエントリを返す(t *testing.T) {
	queue := NewQueue()
	enqueueTest(t, &queue, "e1", "1", nil, []string{"a.go"})
	enqueueTest(t, &queue, "e2", "2", []string{"a.go"}, nil)
	enqueueTest(t, &queue, "e3", "3", nil, []string{"b.go"})
	enqueueTest(t, &queue, "e4", "4", []string{"a.go", "b.go"}, nil)
	running, err := NewCellGroup("g1", "1", "sample", "feat", Git, None, NoNotification, "e1")
	if err != nil {
		t.Fatal(err)
	}
	direct, err := NewCellGroup("g9", "9", "sample", "feat", Git, None, NoNotification, "")
	if err != nil {
		t.Fatal(err)
	}
	groups := []CellGroup{direct, running}
	if got := issuesOf(SelectStartableQueueEntriesService(queue, groups)); !reflect.DeepEqual(got, []string{"3"}) {
		t.Fatalf("startable = %#v, want [3]", got)
	}
	if err := queue.Remove("e1"); err != nil {
		t.Fatal(err)
	}
	if got := issuesOf(SelectStartableQueueEntriesService(queue, groups)); !reflect.DeepEqual(got, []string{"2", "3"}) {
		t.Fatalf("startable after remove = %#v, want [2 3]", got)
	}
}

func TestFindQueueEntryCellGroupServiceはエントリから実行中のCellGroupを引く(t *testing.T) {
	direct, _ := NewCellGroup("g1", "1", "sample", "feat", Git, None, NoNotification, "")
	queued, _ := NewCellGroup("g2", "2", "sample", "feat", Git, None, NoNotification, "e2")
	groups := []CellGroup{direct, queued}
	if group, ok := FindQueueEntryCellGroupService(groups, "e2"); !ok || group.ID != "g2" {
		t.Fatalf("group = %#v, ok = %v", group, ok)
	}
	if _, ok := FindQueueEntryCellGroupService(groups, "e1"); ok {
		t.Fatal("waiting entry found a group")
	}
	if _, ok := FindQueueEntryCellGroupService(groups, ""); ok {
		t.Fatal("empty entry id matched a direct fork")
	}
}

func TestQueueFactoryRestoreは保存値からQueueを戻し不正な値を拒否する(t *testing.T) {
	raw := []RestoredQueueEntry{
		{ID: "e2", Issue: "2", Modify: []string{"a.go"}, Template: "feat", Sequence: 5},
		{ID: "e1", Issue: "1", Create: []string{"a.go"}, Template: "feat", Prefix: "fix", Command: "run", Note: "note", Sequence: 3},
	}
	queue, err := QueueFactory{}.Restore(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got := issuesOf(queue.Entries); !reflect.DeepEqual(got, []string{"1", "2"}) {
		t.Fatalf("entries are not in sequence order: %#v", got)
	}
	if queue.Entries[0].Request != (ForkRequest{Template: "feat", Prefix: "fix", Command: "run", Note: "note"}) {
		t.Fatalf("request = %#v", queue.Entries[0].Request)
	}
	bad := map[string]RestoredQueueEntry{
		"no sequence": {ID: "e3", Issue: "3", Create: []string{"b.go"}, Template: "feat"},
		"same issue":  {ID: "e3", Issue: "1", Create: []string{"b.go"}, Template: "feat", Sequence: 9},
		"bad path":    {ID: "e3", Issue: "3", Create: []string{"../b.go"}, Template: "feat", Sequence: 9},
		"no paths":    {ID: "e3", Issue: "3", Template: "feat", Sequence: 9},
		"no template": {ID: "e3", Issue: "3", Create: []string{"b.go"}, Sequence: 9},
	}
	for name, entry := range bad {
		if _, err := (QueueFactory{}).Restore(append(append([]RestoredQueueEntry(nil), raw...), entry)); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
}
