package domain

import "testing"

func TestCellGroupPreservesNotificationDefaultAndValidatesIdentity(t *testing.T) {
	group, err := NewCellGroup("group", "118", "sample", "feat", Git, None, "")
	if err != nil {
		t.Fatal(err)
	}
	commander, err := NewCommanderCell("commander", group.ID, NewWorkspace(Tmux, nil))
	if err != nil {
		t.Fatal(err)
	}
	if group.ResourceDrivers(commander.Workspace.Driver).Notification != NoNotification {
		t.Fatalf("notification driver = %q", group.ResourceDrivers(commander.Workspace.Driver).Notification)
	}
	for _, input := range [][3]string{{"", "118", "feat"}, {"group", "", "feat"}, {"group", "118", ""}} {
		if _, err := NewCellGroup(input[0], input[1], "sample", input[2], Git, None, NoNotification); err == nil {
			t.Fatalf("invalid identity accepted: %v", input)
		}
	}
}

func TestCommanderStoresCellGroupID(t *testing.T) {
	group, _ := NewCellGroup("group", "118", "sample", "feat", Git, None, NoNotification)
	cell, _ := NewCommanderCell("commander", group.ID, NewWorkspace(Tmux, nil))
	clone := cell.Clone()
	if clone.CellGroupID != group.ID || cell.CellGroupID != group.ID {
		t.Fatalf("CellGroup references = %q and %q, want %q", clone.CellGroupID, cell.CellGroupID, group.ID)
	}
}
