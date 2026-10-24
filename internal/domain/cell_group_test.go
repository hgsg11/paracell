package domain

import (
	"errors"
	"reflect"
	"testing"
)

func newTestCellGroup(t *testing.T, issue string) CellGroup {
	t.Helper()
	group, err := NewCellGroup("group", issue, "My App", "feat", Git, Docker, NoNotification)
	if err != nil {
		t.Fatal(err)
	}
	return group
}

func TestCellGroupPreservesNotificationDefaultAndValidatesIdentity(t *testing.T) {
	group, err := NewCellGroup("group", "118", "sample", "feat", Git, None, "")
	if err != nil {
		t.Fatal(err)
	}
	if group.ResourceDrivers(Tmux).Notification != NoNotification {
		t.Fatalf("notification driver = %q", group.ResourceDrivers(Tmux).Notification)
	}
	if group.Creation != NewCellCreation() {
		t.Fatalf("creation = %#v", group.Creation)
	}
	for _, input := range [][3]string{{"", "118", "feat"}, {"group", "", "feat"}, {"group", "118", ""}} {
		if _, err := NewCellGroup(input[0], input[1], "sample", input[2], Git, None, NoNotification); err == nil {
			t.Fatalf("invalid identity accepted: %v", input)
		}
	}
	if _, err := NewCellGroup("group", "118", "sample", "feat", Git, "podman", NoNotification); err == nil {
		t.Fatal("invalid container driver accepted")
	}
}

func TestCellGroupの作成状態は開始失敗完了で遷移する(t *testing.T) {
	group := newTestCellGroup(t, "118")
	group.BeginCreation()
	if group.Creation != (CellCreation{Status: CreationCreating}) {
		t.Fatalf("begin = %#v", group.Creation)
	}
	group.FailCreation(CreationStageContainers, errors.New("docker failed"))
	if group.Creation != (CellCreation{Status: CreationFailed, FailedStage: CreationStageContainers, LastError: "docker failed"}) {
		t.Fatalf("fail = %#v", group.Creation)
	}
	group.FailCreation(CreationStageWorkspace, nil)
	if group.Creation != (CellCreation{Status: CreationFailed, FailedStage: CreationStageWorkspace}) {
		t.Fatalf("fail without error = %#v", group.Creation)
	}
	group.FinishCreation()
	if group.Creation != (CellCreation{Status: CreationReady}) {
		t.Fatalf("finish = %#v", group.Creation)
	}
	group.FailCreation(CreationStageSource, errors.New("git failed"))
	group.BeginCreation()
	if group.Creation != (CellCreation{Status: CreationCreating}) {
		t.Fatalf("begin after failure kept the old failure: %#v", group.Creation)
	}
}

func TestCellGroupMatchesはIDとIssueとCell名だけを比べる(t *testing.T) {
	group := newTestCellGroup(t, "feat/118 api")
	for _, identifier := range []string{"group", "feat/118 api", "feat-118-api"} {
		if !group.Matches(identifier) {
			t.Fatalf("Matches(%q) = false", identifier)
		}
	}
	for _, identifier := range []string{"commander", "", "118", "feat"} {
		if group.Matches(identifier) {
			t.Fatalf("Matches(%q) = true", identifier)
		}
	}
}

func TestCellGroupのリソース名は変更前と同じ値になる(t *testing.T) {
	group := newTestCellGroup(t, "feat/118 api")
	if got := group.Name().Value; got != "feat-118-api" {
		t.Fatalf("name = %q", got)
	}
	if got := group.ResourcePrefix(); got != "paracell-My-App-feat-118-api" {
		t.Fatalf("resource prefix = %q", got)
	}
	if got := group.WorkspaceName(); got != "My-App-feat-118-api" {
		t.Fatalf("workspace name = %q", got)
	}
	if got := group.SourceWorktreePath("web app"); got != ".paracell/cells/feat-118-api/web-app/source" {
		t.Fatalf("source worktree path = %q", got)
	}
	if got := group.SourceWorktreePath("///"); got != ".paracell/cells/feat-118-api/target/source" {
		t.Fatalf("fallback source worktree path = %q", got)
	}
}

func TestCellGroupResourceDriversは引数のWorkspaceドライバを使う(t *testing.T) {
	group := newTestCellGroup(t, "118")
	group.NotificationDriver = TmuxNotification
	want := NewCellDrivers(Git, Docker, Tmux, TmuxNotification)
	if got := group.ResourceDrivers(Tmux); got != want {
		t.Fatalf("drivers = %#v, want %#v", got, want)
	}
	if got := group.ResourceDrivers("other"); got.Workspace != "other" {
		t.Fatalf("workspace driver = %q", got.Workspace)
	}
}

func TestCellGroupWorkspaceResourceはNoteをラベルにしなければCell名にする(t *testing.T) {
	group := newTestCellGroup(t, "118")
	windows := []WorkspaceWindow{{Name: "agent", Command: "codex"}}
	workspace := NewWorkspace(Tmux, windows)
	want := NewWorkspaceResource("My-App-118", "118", "My App", "118", ".paracell/cells/118", windows)
	if got := group.WorkspaceResource(workspace); !reflect.DeepEqual(got, want) {
		t.Fatalf("resource = %#v, want %#v", got, want)
	}
	if err := group.SetNote(" API\t実装 "); err != nil {
		t.Fatal(err)
	}
	resource := group.WorkspaceResource(workspace)
	if resource.DisplayLabel != "API 実装" || resource.CellName != "118" {
		t.Fatalf("resource = %#v", resource)
	}
	resource.Windows[0].Command = "changed"
	if workspace.Windows[0].Command != "codex" {
		t.Fatal("the resource shares the CommanderCell windows")
	}
}

func TestCellGroupSetNoteは不正なNoteを拒否して元のNoteを保つ(t *testing.T) {
	group := newTestCellGroup(t, "118")
	if err := group.SetNote("first"); err != nil {
		t.Fatal(err)
	}
	for _, note := range []string{"", " \t\n", "123456789012345678901"} {
		if err := group.SetNote(note); err == nil {
			t.Fatalf("SetNote(%q) returned no error", note)
		}
	}
	if group.Note != "first" {
		t.Fatalf("note = %q", group.Note)
	}
}
