package domain

import "testing"

func TestCellGroupFactoryRestoreは保存値からCellGroupを戻す(t *testing.T) {
	group, err := CellGroupFactory{}.Restore("group", "118", "sample", "feat", "", "git", "docker", "tmux", "ready", "", "")
	if err != nil {
		t.Fatal(err)
	}
	want := CellGroup{
		ID: "group", Issue: "118", Project: "sample", Template: "feat",
		SourceDriver: Git, ContainerDriver: Docker, NotificationDriver: TmuxNotification,
		Creation: CellCreation{Status: CreationReady},
	}
	if group != want {
		t.Fatalf("group = %#v, want %#v", group, want)
	}
}

func TestCellGroupFactoryRestoreはNoteを正規化し通知の既定値を戻す(t *testing.T) {
	group, err := CellGroupFactory{}.Restore("group", "118", "sample", "feat", "  API\t実装 ", "git", "none", "", "creating", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if group.Note != "API 実装" || group.NotificationDriver != NoNotification || group.Creation.Status != CreationCreating {
		t.Fatalf("group = %#v", group)
	}
}

func TestCellGroupFactoryRestoreは作成失敗の段階とエラーを戻す(t *testing.T) {
	group, err := CellGroupFactory{}.Restore("group", "118", "sample", "feat", "", "git", "none", "none", "failed", "containers", "docker failed")
	if err != nil {
		t.Fatal(err)
	}
	if group.Creation != (CellCreation{Status: CreationFailed, FailedStage: CreationStageContainers, LastError: "docker failed"}) {
		t.Fatalf("creation = %#v", group.Creation)
	}
}

func TestCellGroupFactoryRestoreは失敗していないとき失敗段階を持たない(t *testing.T) {
	group, err := CellGroupFactory{}.Restore("group", "118", "sample", "feat", "", "git", "none", "none", "ready", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if group.Creation.FailedStage != "" || group.Creation.LastError != "" {
		t.Fatalf("creation = %#v", group.Creation)
	}
}

func TestCellGroupFactoryRestoreは不正な値を拒否する(t *testing.T) {
	cases := map[string][11]string{
		"missing id":            {"", "118", "sample", "feat", "", "git", "none", "none", "ready", "", ""},
		"missing issue":         {"group", "", "sample", "feat", "", "git", "none", "none", "ready", "", ""},
		"missing template":      {"group", "118", "sample", "", "", "git", "none", "none", "ready", "", ""},
		"source driver":         {"group", "118", "sample", "feat", "", "svn", "none", "none", "ready", "", ""},
		"container driver":      {"group", "118", "sample", "feat", "", "git", "podman", "none", "ready", "", ""},
		"notification driver":   {"group", "118", "sample", "feat", "", "git", "none", "mail", "ready", "", ""},
		"note too long":         {"group", "118", "sample", "feat", "123456789012345678901", "git", "none", "none", "ready", "", ""},
		"creation status":       {"group", "118", "sample", "feat", "", "git", "none", "none", "done", "", ""},
		"empty creation status": {"group", "118", "sample", "feat", "", "git", "none", "none", "", "", ""},
		"failed without stage":  {"group", "118", "sample", "feat", "", "git", "none", "none", "failed", "", "boom"},
		"failed with bad stage": {"group", "118", "sample", "feat", "", "git", "none", "none", "failed", "deploy", "boom"},
		"ready with bad stage":  {"group", "118", "sample", "feat", "", "git", "none", "none", "ready", "deploy", ""},
	}
	for name, v := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := (CellGroupFactory{}).Restore(v[0], v[1], v[2], v[3], v[4], v[5], v[6], v[7], v[8], v[9], v[10]); err == nil {
				t.Fatal("invalid value accepted")
			}
		})
	}
}
