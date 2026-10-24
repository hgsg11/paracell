package state

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/hgsg11/paracell/internal/domain"
	"github.com/hgsg11/paracell/internal/usecase"
)

func newTestAdapter(t *testing.T) SQLiteCellAdapter {
	t.Helper()
	return NewSQLiteCellAdapter(filepath.Join(t.TempDir(), "state.db"))
}

// persistedTestCell returns an unsaved CellGroup and its CommanderCell.
func persistedTestCell(t *testing.T, groupID string, commanderID string, issue string) (domain.CellGroup, domain.CommanderCell) {
	t.Helper()
	group, err := domain.NewCellGroup(groupID, issue, "sample", "feat", domain.Git, domain.Docker, domain.NoNotification)
	if err != nil {
		t.Fatal(err)
	}
	window, err := domain.NewWorkspaceWindow("agent", "codex implement")
	if err != nil {
		t.Fatal(err)
	}
	cell, err := domain.NewCommanderCell(commanderID, group.ID, domain.NewWorkspace(domain.Tmux, []domain.WorkspaceWindow{window}))
	if err != nil {
		t.Fatal(err)
	}
	return group, cell
}

func readRecord(t *testing.T, adapter SQLiteCellAdapter, id string) (uint64, map[string]any) {
	t.Helper()
	db, err := adapter.open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var version uint64
	var data []byte
	if err := db.QueryRowContext(context.Background(), "SELECT version, record FROM cells WHERE id = ?", id).Scan(&version, &data); err != nil {
		t.Fatal(err)
	}
	var record map[string]any
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	return version, record
}

func TestSQLiteStateは新しい形式で保存し読み込みで同じCellGroupとCellを戻す(t *testing.T) {
	ctx := context.Background()
	adapter := newTestAdapter(t)
	group, commander := persistedTestCell(t, "group-id", "id", "42")
	if err := group.SetNote("API 実装"); err != nil {
		t.Fatal(err)
	}
	group.FailCreation(domain.CreationStageContainers, errors.New("docker failed"))
	commander.MarkPending()
	source, _ := domain.NewSource(".", "main", "feat/42")
	web, _ := domain.NewContainer([]string{"net"}, "web", domain.Target)
	target, _ := domain.NewTargetCell("api", group.ID, "api", &source, []*domain.Container{&web})
	postgres, _ := domain.NewContainer([]string{"net"}, "postgres", domain.Dependency)
	dependency, _ := domain.NewDependencyCell("db", group.ID, "database", postgres)
	want := usecase.NewCellSet([]domain.CellGroup{group}, []domain.CommanderCell{commander}, []domain.TargetCell{target}, []domain.DependencyCell{dependency})
	if err := adapter.UpdateCells(ctx, func(usecase.CellSet) (usecase.CellSet, error) { return want, nil }); err != nil {
		t.Fatal(err)
	}
	got, err := adapter.LoadCells(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want.Commanders[0].Version = 1
	if !reflect.DeepEqual(got.Groups, want.Groups) || !reflect.DeepEqual(got.Commanders, want.Commanders) {
		t.Fatalf("got %#v / %#v, want %#v / %#v", got.Groups, got.Commanders, want.Groups, want.Commanders)
	}
	if !reflect.DeepEqual(got.Targets, want.Targets) || !reflect.DeepEqual(got.Dependencies, want.Dependencies) {
		t.Fatalf("members = %#v / %#v", got.Targets, got.Dependencies)
	}
	version, record := readRecord(t, adapter, "id")
	stored := record["commander"].(map[string]any)
	if version != 1 || record["cellGroup"] == nil || stored["cellGroupId"] != "group-id" || stored["cellGroup"] != nil || stored["version"] != nil {
		t.Fatalf("record = %v, version = %d", record, version)
	}
}

func TestSQLiteStateは更新のたびにSQLで版を1進め読み込みと変更なしでは進めない(t *testing.T) {
	ctx := context.Background()
	adapter := newTestAdapter(t)
	group, commander := persistedTestCell(t, "group-id", "id", "42")
	if err := adapter.SaveCells(ctx, []domain.CellGroup{group}, []domain.CommanderCell{commander}); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		got, err := adapter.LoadCells(ctx)
		if err != nil || got.Commanders[0].Version != 1 {
			t.Fatalf("version = %#v, %v", got.Commanders, err)
		}
	}
	if err := adapter.UpdateCells(ctx, func(set usecase.CellSet) (usecase.CellSet, error) {
		return set, set.Groups[0].SetNote("updated")
	}); err != nil {
		t.Fatal(err)
	}
	if err := adapter.UpdateCells(ctx, func(set usecase.CellSet) (usecase.CellSet, error) { return set, nil }); err != nil {
		t.Fatal(err)
	}
	if version, _ := readRecord(t, adapter, "id"); version != 2 {
		t.Fatalf("version = %d", version)
	}
	if err := adapter.UpdateCells(ctx, func(set usecase.CellSet) (usecase.CellSet, error) {
		return set, set.Commanders[0].MarkDone()
	}); err != nil {
		t.Fatal(err)
	}
	got, err := adapter.LoadCells(ctx)
	if err != nil || got.Commanders[0].Version != 3 || !got.Commanders[0].Done || got.Groups[0].Note != "updated" {
		t.Fatalf("got = %#v, %v", got, err)
	}
}

func TestSQLiteStateは古い版のCellの保存をErrVersionConflictで拒否しDBを変えない(t *testing.T) {
	ctx := context.Background()
	adapter := newTestAdapter(t)
	group, commander := persistedTestCell(t, "group-id", "id", "42")
	if err := adapter.SaveCells(ctx, []domain.CellGroup{group}, []domain.CommanderCell{commander}); err != nil {
		t.Fatal(err)
	}
	stale, err := adapter.LoadCells(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := adapter.UpdateCells(ctx, func(set usecase.CellSet) (usecase.CellSet, error) {
		return set, set.Groups[0].SetNote("updated")
	}); err != nil {
		t.Fatal(err)
	}
	// Saving the CommanderCell read before the update (version 1) must fail.
	if err := adapter.UpdateCells(ctx, func(set usecase.CellSet) (usecase.CellSet, error) {
		set.Commanders[0] = stale.Commanders[0]
		set.Commanders[0].MarkPending()
		return set, nil
	}); !errors.Is(err, domain.ErrVersionConflict) {
		t.Fatalf("error = %v", err)
	}
	if err := adapter.SaveCells(ctx, stale.Groups, stale.Commanders); !errors.Is(err, domain.ErrVersionConflict) {
		t.Fatalf("error = %v", err)
	}
	got, err := adapter.LoadCells(ctx)
	if err != nil || got.Commanders[0].Version != 2 || got.Commanders[0].Status != domain.Ready || got.Groups[0].Note != "updated" {
		t.Fatalf("state changed after conflict: %#v, %v", got, err)
	}
}

func TestSQLiteStateは版を持つ新しいCellの追加を拒否する(t *testing.T) {
	ctx := context.Background()
	adapter := newTestAdapter(t)
	group, _ := persistedTestCell(t, "group-id", "id", "42")
	restored, err := domain.CommanderCellFactory{}.Restore("id", group.ID, "tmux", nil, "ready", false, 3)
	if err != nil {
		t.Fatal(err)
	}
	if err := adapter.SaveCells(ctx, []domain.CellGroup{group}, []domain.CommanderCell{restored}); err == nil {
		t.Fatal("new CommanderCell with a version was inserted")
	}
	got, err := adapter.LoadCells(ctx)
	if err != nil || len(got.Groups)+len(got.Commanders) != 0 {
		t.Fatalf("got = %#v, %v", got, err)
	}
}

func TestSQLiteStateは途中で失敗した更新をすべて取り消す(t *testing.T) {
	ctx := context.Background()
	adapter := newTestAdapter(t)
	firstGroup, first := persistedTestCell(t, "group-1", "commander-1", "42")
	secondGroup, second := persistedTestCell(t, "group-2", "commander-2", "43")
	if err := adapter.SaveCells(ctx, []domain.CellGroup{firstGroup, secondGroup}, []domain.CommanderCell{first, second}); err != nil {
		t.Fatal(err)
	}
	stale, err := adapter.LoadCells(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := adapter.UpdateCells(ctx, func(set usecase.CellSet) (usecase.CellSet, error) {
		return set, set.Groups[1].SetNote("second")
	}); err != nil {
		t.Fatal(err)
	}
	// The first row is written before the stale second row fails its version check.
	if err := adapter.UpdateCells(ctx, func(set usecase.CellSet) (usecase.CellSet, error) {
		if err := set.Groups[0].SetNote("first"); err != nil {
			return set, err
		}
		set.Commanders[1] = stale.Commanders[1]
		set.Commanders[1].MarkPending()
		return set, nil
	}); !errors.Is(err, domain.ErrVersionConflict) {
		t.Fatalf("error = %v", err)
	}
	got, err := adapter.LoadCells(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.Groups[0].Note != "" || got.Commanders[0].Version != 1 || got.Groups[1].Note != "second" || got.Commanders[1].Status != domain.Ready {
		t.Fatalf("partial update persisted: %#v", got)
	}
}

func TestDeleteCellGroupPreservesOtherGroups(t *testing.T) {
	ctx := context.Background()
	adapter := newTestAdapter(t)
	firstGroup, first := persistedTestCell(t, "group-1", "commander-1", "42")
	secondGroup, second := persistedTestCell(t, "group-2", "commander-2", "43")
	if err := adapter.SaveCells(ctx, []domain.CellGroup{firstGroup, secondGroup}, []domain.CommanderCell{first, second}); err != nil {
		t.Fatal(err)
	}
	if err := adapter.UpdateCells(ctx, func(set usecase.CellSet) (usecase.CellSet, error) {
		return usecase.NewCellSet(set.Groups[1:], set.Commanders[1:], nil, nil), nil
	}); err != nil {
		t.Fatal(err)
	}
	got, err := adapter.LoadCells(ctx)
	if err != nil || len(got.Commanders) != 1 || got.Commanders[0].ID != second.ID || got.Groups[0].ID != secondGroup.ID {
		t.Fatalf("remaining groups = %#v, %v", got, err)
	}
	// The surviving row moved to the released position; adding another uses the next one.
	thirdGroup, third := persistedTestCell(t, "group-3", "commander-3", "44")
	if err := adapter.UpdateCells(ctx, func(set usecase.CellSet) (usecase.CellSet, error) {
		return usecase.NewCellSet(append(set.Groups, thirdGroup), append(set.Commanders, third), nil, nil), nil
	}); err != nil {
		t.Fatal(err)
	}
	got, err = adapter.LoadCells(ctx)
	if err != nil || len(got.Groups) != 2 || got.Groups[1].ID != "group-3" {
		t.Fatalf("groups = %#v, %v", got, err)
	}
}

func TestUpdateCellsは整合しないCellSetを拒否する(t *testing.T) {
	ctx := context.Background()
	adapter := newTestAdapter(t)
	group, commander := persistedTestCell(t, "group-id", "id", "42")
	source, _ := domain.NewSource(".", "main", "feat/42")
	target, _ := domain.NewTargetCell("api", group.ID, "api", &source, nil)
	postgres, _ := domain.NewContainer(nil, "postgres", domain.Dependency)
	dependency, _ := domain.NewDependencyCell("db", group.ID, "database", postgres)
	if err := adapter.UpdateCells(ctx, func(usecase.CellSet) (usecase.CellSet, error) {
		return usecase.NewCellSet([]domain.CellGroup{group}, []domain.CommanderCell{commander}, []domain.TargetCell{target}, []domain.DependencyCell{dependency}), nil
	}); err != nil {
		t.Fatal(err)
	}
	otherGroup, otherCommander := persistedTestCell(t, "other-group", "other", "43")
	cases := map[string]func(usecase.CellSet) usecase.CellSet{
		"CommanderCell with unknown CellGroup": func(set usecase.CellSet) usecase.CellSet {
			set.Commanders[0].CellGroupID = "missing"
			return set
		},
		"TargetCell with unknown CellGroup": func(set usecase.CellSet) usecase.CellSet {
			set.Targets[0].CellGroupID = "missing"
			return set
		},
		"DependencyCell with unknown CellGroup": func(set usecase.CellSet) usecase.CellSet {
			set.Dependencies[0].CellGroupID = "missing"
			return set
		},
		"two CommanderCells in one CellGroup": func(set usecase.CellSet) usecase.CellSet {
			otherCommander.CellGroupID = group.ID
			set.Commanders = append(set.Commanders, otherCommander)
			return set
		},
		"CellGroup without CommanderCell": func(set usecase.CellSet) usecase.CellSet {
			set.Groups = append(set.Groups, otherGroup)
			return set
		},
		"duplicate CellGroup id": func(set usecase.CellSet) usecase.CellSet {
			set.Groups = append(set.Groups, set.Groups[0])
			return set
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			if err := adapter.UpdateCells(ctx, func(set usecase.CellSet) (usecase.CellSet, error) { return change(set), nil }); err == nil {
				t.Fatal("inconsistent CellSet accepted")
			}
			got, err := adapter.LoadCells(ctx)
			if err != nil || len(got.Groups) != 1 || len(got.Commanders) != 1 || got.Commanders[0].Version != 1 {
				t.Fatalf("state changed: %#v, %v", got, err)
			}
		})
	}
}
