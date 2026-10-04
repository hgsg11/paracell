package state

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hgsg11/paracell/internal/domain"
	"github.com/hgsg11/paracell/internal/usecase"
)

func TestCellGroupRoundTripAndRejectedUpdateIsolation(t *testing.T) {
	ctx := context.Background()
	adapter := NewSQLiteCellAdapter(filepath.Join(t.TempDir(), "state.db"))
	commander := persistedTestCell(t)
	source, _ := domain.NewSource(".", "main", "feat/42")
	container, _ := domain.NewContainer([]string{"original"}, "postgres", domain.Dependency)
	dependency, _ := domain.NewDependencyCell("db", commander.CellGroup.ID, "database", container)
	target, _ := domain.NewTargetCell("api", commander.CellGroup.ID, "api", &source, nil, []string{dependency.ID})
	want := usecase.NewCellSet([]domain.CommanderCell{commander}, []domain.TargetCell{target}, []domain.DependencyCell{dependency})
	if err := adapter.UpdateCells(ctx, func(usecase.CellSet) (usecase.CellSet, error) { return want, nil }); err != nil {
		t.Fatal(err)
	}
	got, err := adapter.LoadCells(ctx)
	if err != nil {
		t.Fatal(err)
	}
	targets, dependencies := domain.SelectCellGroupMembersService(got.Commanders[0].CellGroup.ID, got.Targets, got.Dependencies)
	if len(targets) != 1 || len(dependencies) != 1 || targets[0].Dependencies[0] != dependencies[0].ID || got.Commanders[0].CellGroup.ID == got.Commanders[0].ID {
		t.Fatalf("group associations were not preserved: %#v", got)
	}
	rejected := errors.New("reject")
	if err := adapter.UpdateCells(ctx, func(set usecase.CellSet) (usecase.CellSet, error) {
		if err := set.Commanders[0].CellGroup.SetNote("not saved"); err != nil {
			t.Fatal(err)
		}
		return set, rejected
	}); !errors.Is(err, rejected) {
		t.Fatalf("error = %v", err)
	}
	got, err = adapter.LoadCells(ctx)
	if err != nil || got.Commanders[0].CellGroup.Note != "" {
		t.Fatalf("rejected note persisted: %#v, %v", got, err)
	}
	if err := adapter.UpdateCells(ctx, func(set usecase.CellSet) (usecase.CellSet, error) {
		set.Targets[0].CellGroupID = "missing"
		return set, nil
	}); err == nil {
		t.Fatal("unknown group accepted")
	}
}

const legacyGroupRecord = `{
 "commander": {
  "version": 3, "id": "old", "issue": "42", "project": "sample", "note": "old note", "template": "feat",
  "workspace": {"Driver":"tmux","Windows":[{"Name":"agent","Command":"codex implement"}]},
  "targets":["api"], "dependencies":["db"], "sourceDriver":"git", "containerDriver":"docker", "notificationDriver":"none",
  "creation":{"Status":"ready"}, "status":"pending", "done":true
 },
 "targets":[{"ID":"api","CommanderID":"old","Name":"api","Source":{"Path":".","Base":"main","Branch":"feat/42"},"Dependencies":["db"]}],
 "dependencies":[{"ID":"db","CommanderID":"old","Name":"database","Container":{"SourceContainer":"postgres","Mode":"dependency","Network":["original"]}}]
}`

func TestLegacyCommanderRecordLoadsAndUpdatesAsCellGroup(t *testing.T) {
	ctx := context.Background()
	adapter := NewSQLiteCellAdapter(filepath.Join(t.TempDir(), "state.db"))
	db, err := adapter.open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, "INSERT INTO cells VALUES (?, ?, ?, ?, ?)", "old", "42", 0, 3, []byte(legacyGroupRecord)); err != nil {
		t.Fatal(err)
	}
	set, err := adapter.LoadCells(ctx)
	if err != nil {
		t.Fatal(err)
	}
	cell := set.Commanders[0]
	if cell.CellGroup.ID != "old" || cell.CellGroup.Note != "old note" || cell.Version != 3 || cell.Status != domain.Pending || !cell.Done || cell.Workspace.Windows[0].Command != "codex implement" {
		t.Fatalf("legacy runtime information changed: %#v", cell)
	}
	if set.Targets[0].CellGroupID != "old" || set.Dependencies[0].CellGroupID != "old" || set.Targets[0].Dependencies[0] != "db" || set.Dependencies[0].Container.Network[0] != "original" {
		t.Fatalf("legacy membership/resources changed: %#v", set)
	}
	if err := adapter.UpdateCells(ctx, func(set usecase.CellSet) (usecase.CellSet, error) {
		return set, set.Commanders[0].CellGroup.SetNote("new note")
	}); err != nil {
		t.Fatal(err)
	}
	var data []byte
	if err := db.QueryRowContext(ctx, "SELECT record FROM cells").Scan(&data); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "CommanderID") || !strings.Contains(string(data), "cellGroup") {
		t.Fatalf("not rewritten: %s", data)
	}
	set, err = adapter.LoadCells(ctx)
	if err != nil || set.Commanders[0].CellGroup.Note != "new note" || set.Commanders[0].Version != 4 {
		t.Fatalf("updated group = %#v, error = %v", set, err)
	}
}

func TestLegacyCommanderRejectsBrokenMembership(t *testing.T) {
	for _, data := range []string{
		strings.Replace(legacyGroupRecord, `"CommanderID":"old"`, `"CommanderID":"other"`, 1),
		strings.Replace(legacyGroupRecord, `"targets":["api"]`, `"targets":["missing"]`, 1),
		strings.Replace(legacyGroupRecord, `"dependencies":["db"]`, `"dependencies":["missing"]`, 1),
	} {
		if _, err := decodeCellRecord([]byte(data)); err == nil {
			t.Fatal("broken legacy association accepted")
		}
	}
}

func TestDeleteCellGroupPreservesOtherGroups(t *testing.T) {
	ctx := context.Background()
	adapter := NewSQLiteCellAdapter(filepath.Join(t.TempDir(), "state.db"))
	first := persistedTestCell(t)
	group, _ := domain.NewCellGroup("another-group", "43", "sample", "feat", domain.Git, domain.None, domain.NoNotification)
	second, _ := domain.NewCommanderCell("another-commander", &group, domain.NewWorkspace(domain.Tmux, nil))
	if err := adapter.SaveCells(ctx, []domain.CommanderCell{first, second}); err != nil {
		t.Fatal(err)
	}
	if err := adapter.UpdateCells(ctx, func(set usecase.CellSet) (usecase.CellSet, error) {
		return usecase.NewCellSet(set.Commanders[1:], nil, nil), nil
	}); err != nil {
		t.Fatal(err)
	}
	got, err := adapter.LoadCells(ctx)
	if err != nil || len(got.Commanders) != 1 || !got.Commanders[0].SameIdentity(second) {
		t.Fatalf("remaining groups = %#v, %v", got, err)
	}
}
