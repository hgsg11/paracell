package state

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/hgsg11/paracell/internal/domain"
	"github.com/hgsg11/paracell/internal/usecase"
)

func TestCellGroupRoundTripAndRejectedUpdateIsolation(t *testing.T) {
	ctx := context.Background()
	adapter := newTestAdapter(t)
	group, commander := persistedTestCell(t, "group-id", "id", "42")
	source, _ := domain.NewSource(".", "main", "feat/42")
	container, _ := domain.NewContainer([]string{"original"}, "postgres", domain.Dependency)
	dependency, _ := domain.NewDependencyCell("db", group.ID, "database", container)
	target, _ := domain.NewTargetCell("api", group.ID, "api", &source, nil)
	want := usecase.NewCellSet([]domain.CellGroup{group}, []domain.CommanderCell{commander}, []domain.TargetCell{target}, []domain.DependencyCell{dependency})
	if err := adapter.UpdateCells(ctx, func(usecase.CellSet) (usecase.CellSet, error) { return want, nil }); err != nil {
		t.Fatal(err)
	}
	got, err := adapter.LoadCells(ctx)
	if err != nil {
		t.Fatal(err)
	}
	targets, dependencies := domain.SelectCellGroupMembersService(got.Commanders[0].CellGroupID, got.Targets, got.Dependencies)
	if len(targets) != 1 || len(dependencies) != 1 || got.Groups[0].ID == got.Commanders[0].ID {
		t.Fatalf("group associations were not preserved: %#v", got)
	}
	rejected := errors.New("reject")
	if err := adapter.UpdateCells(ctx, func(set usecase.CellSet) (usecase.CellSet, error) {
		if err := set.Groups[0].SetNote("not saved"); err != nil {
			t.Fatal(err)
		}
		return set, rejected
	}); !errors.Is(err, rejected) {
		t.Fatalf("error = %v", err)
	}
	got, err = adapter.LoadCells(ctx)
	if err != nil || got.Groups[0].Note != "" {
		t.Fatalf("rejected note persisted: %#v, %v", got, err)
	}
}

func TestDecodeCellRecordPreservesExistingTargetContainer(t *testing.T) {
	data, err := json.Marshal(map[string]any{
		"commander": map[string]any{
			"Version": 1, "ID": "commander", "CellGroup": map[string]any{
				"ID": "group", "Issue": "42", "Project": "sample", "Template": "feat",
				"SourceDriver": "git", "ContainerDriver": "docker", "NotificationDriver": "none",
			},
			"Workspace": map[string]any{"Driver": "tmux"}, "Creation": map[string]any{"Status": "ready"}, "Status": "ready",
		},
		"targets": []any{map[string]any{
			"ID": "target", "CellGroupID": "group", "Name": "app",
			"Container":    map[string]any{"Network": []string{"old-network"}, "SourceContainer": "web", "Mode": "target"},
			"Dependencies": []string{"dependency"},
		}},
		"dependencies": []any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeCellRecord(data, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded.Targets) != 1 || len(decoded.Targets[0].Containers) != 1 || decoded.Targets[0].Containers[0].SourceContainer != "web" {
		t.Fatalf("existing TargetCell container was lost: %#v", decoded.Targets)
	}
	if _, err := decodeCellRecord(data, 2); err == nil {
		t.Fatal("record version that differs from its row was accepted")
	}
}

// currentGroupRecord is a row written by the previous format: the CellGroup and
// the creation state are nested in "commander" with Go field names.
const currentGroupRecord = `{
 "commander": {
  "version": 2, "id": "cur",
  "cellGroup": {"ID":"grp","Note":"current note","Issue":"43","Project":"sample","Template":"feat","NotificationDriver":"tmux","SourceDriver":"git","ContainerDriver":"docker"},
  "workspace": {"Driver":"tmux","Windows":[{"Name":"agent","Command":"codex implement"}]},
  "creation": {"Status":"failed","FailedStage":"containers","LastError":"docker failed"},
  "status": "pending", "done": true
 },
 "targets": [{"ID":"api","CellGroupID":"grp","Name":"api","Source":{"Path":".","Base":"main","Branch":"feat/43"},"Containers":[{"Network":["net"],"SourceContainer":"web","Mode":"target"}]}],
 "dependencies": [{"ID":"db","CellGroupID":"grp","Name":"database","Container":{"Network":["net"],"SourceContainer":"postgres","Mode":"dependency"}}]
}`

// legacyGroupRecord is a row written before CellGroup existed.
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

func insertRawRow(t *testing.T, adapter SQLiteCellAdapter, id string, issue string, position int, version uint64, record string) {
	t.Helper()
	db, err := adapter.open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.ExecContext(context.Background(), "INSERT INTO cells VALUES (?, ?, ?, ?, ?)", id, issue, position, version, []byte(record)); err != nil {
		t.Fatal(err)
	}
}

func rawRecord(t *testing.T, adapter SQLiteCellAdapter, id string) string {
	t.Helper()
	db, err := adapter.open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var data []byte
	if err := db.QueryRowContext(context.Background(), "SELECT record FROM cells WHERE id = ?", id).Scan(&data); err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestCurrentFormatRecordLoadsAndUpdatesInNewFormat(t *testing.T) {
	ctx := context.Background()
	adapter := newTestAdapter(t)
	insertRawRow(t, adapter, "cur", "43", 0, 2, currentGroupRecord)
	set, err := adapter.LoadCells(ctx)
	if err != nil {
		t.Fatal(err)
	}
	group, cell := set.Groups[0], set.Commanders[0]
	wantCreation := domain.CellCreation{Status: domain.CreationFailed, FailedStage: domain.CreationStageContainers, LastError: "docker failed"}
	if group.ID != "grp" || group.Issue != "43" || group.Note != "current note" || group.NotificationDriver != domain.TmuxNotification || group.ContainerDriver != domain.Docker || group.Creation != wantCreation {
		t.Fatalf("CellGroup changed: %#v", group)
	}
	if cell.ID != "cur" || cell.CellGroupID != "grp" || cell.Version != 2 || cell.Status != domain.Pending || !cell.Done || cell.Workspace.Windows[0].Command != "codex implement" {
		t.Fatalf("CommanderCell changed: %#v", cell)
	}
	if set.Targets[0].CellGroupID != "grp" || set.Targets[0].Containers[0].Network[0] != "net" || set.Dependencies[0].Container.SourceContainer != "postgres" {
		t.Fatalf("members changed: %#v", set)
	}
	// An update with no change keeps the earlier format.
	if err := adapter.UpdateCells(ctx, func(set usecase.CellSet) (usecase.CellSet, error) { return set, nil }); err != nil {
		t.Fatal(err)
	}
	if got := rawRecord(t, adapter, "cur"); got != currentGroupRecord {
		t.Fatalf("unchanged row was rewritten: %s", got)
	}
	if err := adapter.UpdateCells(ctx, func(set usecase.CellSet) (usecase.CellSet, error) {
		return set, set.Commanders[0].MarkUndone()
	}); err != nil {
		t.Fatal(err)
	}
	version, record := readRecord(t, adapter, "cur")
	if version != 3 || record["cellGroup"] == nil || record["commander"].(map[string]any)["cellGroup"] != nil {
		t.Fatalf("not rewritten in the new format: %v (version %d)", record, version)
	}
	set, err = adapter.LoadCells(ctx)
	if err != nil || set.Commanders[0].Done || set.Groups[0].Note != "current note" || set.Groups[0].Creation != wantCreation || len(set.Targets) != 1 || len(set.Dependencies) != 1 {
		t.Fatalf("rewritten row = %#v, %v", set, err)
	}
}

func TestLegacyCommanderRecordLoadsAndUpdatesAsCellGroup(t *testing.T) {
	ctx := context.Background()
	adapter := newTestAdapter(t)
	insertRawRow(t, adapter, "old", "42", 0, 3, legacyGroupRecord)
	set, err := adapter.LoadCells(ctx)
	if err != nil {
		t.Fatal(err)
	}
	group, cell := set.Groups[0], set.Commanders[0]
	if group.ID != "old" || group.Note != "old note" || group.Issue != "42" || group.Creation.Status != domain.CreationReady {
		t.Fatalf("legacy CellGroup changed: %#v", group)
	}
	if cell.CellGroupID != "old" || cell.Version != 3 || cell.Status != domain.Pending || !cell.Done || cell.Workspace.Windows[0].Command != "codex implement" {
		t.Fatalf("legacy runtime information changed: %#v", cell)
	}
	if set.Targets[0].CellGroupID != "old" || set.Dependencies[0].CellGroupID != "old" || set.Dependencies[0].Container.Network[0] != "original" {
		t.Fatalf("legacy membership/resources changed: %#v", set)
	}
	if err := adapter.UpdateCells(ctx, func(set usecase.CellSet) (usecase.CellSet, error) {
		return set, set.Groups[0].SetNote("new note")
	}); err != nil {
		t.Fatal(err)
	}
	data := rawRecord(t, adapter, "old")
	if strings.Contains(data, "CommanderID") || !strings.Contains(data, `"cellGroup":{`) || !strings.Contains(data, `"cellGroupId":"old"`) {
		t.Fatalf("not rewritten: %s", data)
	}
	set, err = adapter.LoadCells(ctx)
	if err != nil || set.Groups[0].Note != "new note" || set.Commanders[0].Version != 4 || !set.Commanders[0].Done || len(set.Targets) != 1 {
		t.Fatalf("updated group = %#v, error = %v", set, err)
	}
}

func TestLegacyCommanderRejectsBrokenMembership(t *testing.T) {
	for _, data := range []string{
		strings.Replace(legacyGroupRecord, `"CommanderID":"old"`, `"CommanderID":"other"`, 1),
		strings.Replace(legacyGroupRecord, `"targets":["api"]`, `"targets":["missing"]`, 1),
		strings.Replace(legacyGroupRecord, `"dependencies":["db"]`, `"dependencies":["missing"]`, 1),
	} {
		if _, err := decodeCellRecord([]byte(data), 3); err == nil {
			t.Fatal("broken legacy association accepted")
		}
	}
}
