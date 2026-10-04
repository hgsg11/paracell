package domain

import "testing"

func TestCellGroupPreservesNotificationDefaultAndValidatesIdentity(t *testing.T) {
	group, err := NewCellGroup("group", "118", "sample", "feat", Git, None, "")
	if err != nil {
		t.Fatal(err)
	}
	commander, err := NewCommanderCell("commander", &group, NewWorkspace(Tmux, nil))
	if err != nil {
		t.Fatal(err)
	}
	if commander.ResourceDrivers().Notification != NoNotification {
		t.Fatalf("notification driver = %q", commander.ResourceDrivers().Notification)
	}
	for _, input := range [][3]string{{"", "118", "feat"}, {"group", "", "feat"}, {"group", "118", ""}} {
		if _, err := NewCellGroup(input[0], input[1], "sample", input[2], Git, None, NoNotification); err == nil {
			t.Fatalf("invalid identity accepted: %v", input)
		}
	}
}

func TestCommanderCloneIsolatesGroupNote(t *testing.T) {
	group, _ := NewCellGroup("group", "118", "sample", "feat", Git, None, NoNotification)
	cell, _ := NewCommanderCell("commander", &group, NewWorkspace(Tmux, nil))
	clone := cell.Clone()
	if err := clone.CellGroup.SetNote(" API\t実装 "); err != nil {
		t.Fatal(err)
	}
	if label, template := clone.ListLabels(); label != "API 実装" || template != "feat" {
		t.Fatalf("labels = %q, %q", label, template)
	}
	if cell.DisplayLabel() != "118" {
		t.Fatal("updating the snapshot changed the original group")
	}
}
