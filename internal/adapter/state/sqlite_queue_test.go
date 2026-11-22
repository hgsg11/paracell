package state

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/hgsg11/paracell/internal/domain"
	"github.com/hgsg11/paracell/internal/query"
	"github.com/hgsg11/paracell/internal/usecase"
)

func queueTestPaths(t *testing.T, values ...string) []domain.WorkspacePath {
	t.Helper()
	var paths []domain.WorkspacePath
	for _, value := range values {
		path, err := domain.NewWorkspacePath(value)
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}
	return paths
}

// enqueueState adds an entry through UpdateCells, treating the modify paths as on disk.
func enqueueState(t *testing.T, adapter SQLiteCellAdapter, id, issue string, modify []string, create []string, note string) {
	t.Helper()
	plan, err := domain.NewChangePlan(queueTestPaths(t, modify...), queueTestPaths(t, create...))
	if err != nil {
		t.Fatal(err)
	}
	request, err := domain.NewForkRequest("feat", "fix", "run", note)
	if err != nil {
		t.Fatal(err)
	}
	if err := adapter.UpdateCells(context.Background(), func(set usecase.CellSet) (usecase.CellSet, error) {
		_, err := set.Queue.Enqueue(id, issue, plan, request, domain.NewExistingPaths(queueTestPaths(t, modify...)))
		return set, err
	}); err != nil {
		t.Fatal(err)
	}
}

func TestSQLiteStateはQueueとQueueEntryIDを保存し同じ値で戻す(t *testing.T) {
	ctx := context.Background()
	adapter := newTestAdapter(t)
	enqueueState(t, adapter, "e1", "1", []string{"b.go", "a.go"}, []string{"c.go"}, "first")
	enqueueState(t, adapter, "e2", "2", []string{"a.go"}, nil, "")
	group, commander := persistedTestCell(t, "group-1", "commander-1", "1")
	group.QueueEntryID = "e1"
	if err := adapter.UpdateCells(ctx, func(set usecase.CellSet) (usecase.CellSet, error) {
		set.Groups = append(set.Groups, group)
		set.Commanders = append(set.Commanders, commander)
		return set, nil
	}); err != nil {
		t.Fatal(err)
	}
	got, err := adapter.LoadCells(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.Groups[0].QueueEntryID != "e1" {
		t.Fatalf("group = %#v", got.Groups[0])
	}
	if len(got.Queue.Entries) != 2 {
		t.Fatalf("queue = %#v", got.Queue)
	}
	first := got.Queue.Entries[0]
	wantPlan, _ := domain.NewChangePlan(queueTestPaths(t, "b.go", "a.go"), queueTestPaths(t, "c.go"))
	if first.ID != "e1" || first.Issue != "1" || first.Sequence != 1 || !reflect.DeepEqual(first.Plan, wantPlan) ||
		first.Request != (domain.ForkRequest{Template: "feat", Prefix: "fix", Command: "run", Note: "first"}) {
		t.Fatalf("first = %#v", first)
	}
	if got.Queue.Entries[1].Sequence != 2 {
		t.Fatalf("second = %#v", got.Queue.Entries[1])
	}
	_, record := readRecord(t, adapter, "commander-1")
	if record["cellGroup"].(map[string]any)["queueEntryId"] != "e1" {
		t.Fatalf("record = %#v", record)
	}
}

func TestSQLiteStateはfork由来のCellGroupにqueueEntryIdを書かない(t *testing.T) {
	adapter := newTestAdapter(t)
	group, commander := persistedTestCell(t, "group-1", "commander-1", "1")
	if err := adapter.SaveCells(context.Background(), []domain.CellGroup{group}, []domain.CommanderCell{commander}); err != nil {
		t.Fatal(err)
	}
	_, record := readRecord(t, adapter, "commander-1")
	if _, ok := record["cellGroup"].(map[string]any)["queueEntryId"]; ok {
		t.Fatalf("record = %#v", record)
	}
}

func TestSQLiteStateは遷移とエントリの削除を同じトランザクションで保存する(t *testing.T) {
	ctx := context.Background()
	adapter := newTestAdapter(t)
	enqueueState(t, adapter, "e1", "1", []string{"a.go"}, nil, "")
	group, commander := persistedTestCell(t, "group-1", "commander-1", "1")
	group.QueueEntryID = "e1"
	if err := adapter.UpdateCells(ctx, func(set usecase.CellSet) (usecase.CellSet, error) {
		set.Groups = append(set.Groups, group)
		set.Commanders = append(set.Commanders, commander)
		return set, nil
	}); err != nil {
		t.Fatal(err)
	}
	stale, err := adapter.LoadCells(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := adapter.UpdateCells(ctx, func(set usecase.CellSet) (usecase.CellSet, error) {
		set.Commanders[0].MarkPending()
		return set, nil
	}); err != nil {
		t.Fatal(err)
	}
	finish := func(base usecase.CellSet) error {
		return adapter.UpdateCells(ctx, func(set usecase.CellSet) (usecase.CellSet, error) {
			set.Commanders[0] = base.Commanders[0]
			if err := set.Commanders[0].MarkMerged(); err != nil {
				return set, err
			}
			return set, set.Queue.Remove(set.Groups[0].QueueEntryID)
		})
	}
	// A stale CommanderCell fails, and the entry removal is rolled back with it.
	if err := finish(stale); !errors.Is(err, domain.ErrVersionConflict) {
		t.Fatalf("error = %v", err)
	}
	got, err := adapter.LoadCells(ctx)
	if err != nil || got.Commanders[0].Progress != domain.InProgress || len(got.Queue.Entries) != 1 {
		t.Fatalf("partial update persisted: %#v, %v", got, err)
	}
	if err := finish(got); err != nil {
		t.Fatal(err)
	}
	got, err = adapter.LoadCells(ctx)
	if err != nil || got.Commanders[0].Progress != domain.Merged || len(got.Queue.Entries) != 0 {
		t.Fatalf("got = %#v, %v", got, err)
	}
}

func TestSQLiteStateは保存済みのエントリの書き換えを拒否する(t *testing.T) {
	adapter := newTestAdapter(t)
	enqueueState(t, adapter, "e1", "1", []string{"a.go"}, nil, "")
	if err := adapter.UpdateCells(context.Background(), func(set usecase.CellSet) (usecase.CellSet, error) {
		set.Queue.Entries[0].Issue = "changed"
		return set, nil
	}); err == nil {
		t.Fatal("changed entry saved")
	}
}

func TestQueueViewQueryは待ちと実行中と待っている相手を返す(t *testing.T) {
	ctx := context.Background()
	adapter := newTestAdapter(t)
	views, err := adapter.ListQueueViews(ctx)
	if err != nil || len(views) != 0 {
		t.Fatalf("views = %#v, %v", views, err)
	}
	enqueueState(t, adapter, "e1", "1", nil, []string{"internal/new.go"}, "first")
	enqueueState(t, adapter, "e2", "2", []string{"internal/new.go", "README.md"}, nil, "")
	group, commander := persistedTestCell(t, "group-1", "commander-1", "1")
	group.QueueEntryID = "e1"
	if err := adapter.UpdateCells(ctx, func(set usecase.CellSet) (usecase.CellSet, error) {
		set.Groups = append(set.Groups, group)
		set.Commanders = append(set.Commanders, commander)
		return set, nil
	}); err != nil {
		t.Fatal(err)
	}
	views, err = adapter.ListQueueViews(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := []query.QueueView{
		{Issue: "1", Note: "first", State: query.QueueStateRunning, Modify: []string{}, Create: []string{"internal/new.go"}},
		{Issue: "2", State: query.QueueStateWaiting, BlockedBy: []string{"1"}, Modify: []string{"internal/new.go", "README.md"}, Create: []string{}},
	}
	if !reflect.DeepEqual(views, want) {
		t.Fatalf("views = %#v, want %#v", views, want)
	}
	paths, err := adapter.ListPlannedPaths(ctx, "internal/")
	if err != nil {
		t.Fatal(err)
	}
	wantPaths := []query.PlannedPathView{
		{Kind: query.PlannedPathCreate, Path: "internal/new.go", Issue: "1", Note: "first"},
		{Kind: query.PlannedPathModify, Path: "internal/new.go", Issue: "2"},
	}
	if !reflect.DeepEqual(paths, wantPaths) {
		t.Fatalf("paths = %#v, want %#v", paths, wantPaths)
	}
	all, err := adapter.ListPlannedPaths(ctx, "")
	if err != nil || len(all) != 3 || all[0].Path != "README.md" {
		t.Fatalf("all = %#v, %v", all, err)
	}
}
