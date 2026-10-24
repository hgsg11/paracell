package domain

import "testing"

func newTestCommanderCell(t *testing.T) CommanderCell {
	t.Helper()
	window, err := NewWorkspaceWindow("agent", "codex")
	if err != nil {
		t.Fatal(err)
	}
	cell, err := NewCommanderCell("commander", "group", NewWorkspace(Tmux, []WorkspaceWindow{window}))
	if err != nil {
		t.Fatal(err)
	}
	return cell
}

func TestNewCommanderCellはReadyで未完了かつ版なしで作る(t *testing.T) {
	cell := newTestCommanderCell(t)
	if cell.ID != "commander" || cell.CellGroupID != "group" || cell.Status != Ready || cell.Done || cell.Version != 0 {
		t.Fatalf("cell = %#v", cell)
	}
	if cell.Workspace.Driver != Tmux || len(cell.Workspace.Windows) != 1 || cell.Workspace.Windows[0].Command != "codex" {
		t.Fatalf("workspace = %#v", cell.Workspace)
	}
}

func TestNewCommanderCellはIDとCellGroup参照を必須にする(t *testing.T) {
	for _, input := range [][2]string{{"", "group"}, {"commander", ""}, {"", ""}} {
		if _, err := NewCommanderCell(input[0], input[1], NewWorkspace(Tmux, nil)); err == nil {
			t.Fatalf("NewCommanderCell(%q, %q) returned no error", input[0], input[1])
		}
	}
}

func TestNewCommanderCellは渡されたWindowsを共有しない(t *testing.T) {
	windows := []WorkspaceWindow{{Name: "agent", Command: "codex"}}
	cell, err := NewCommanderCell("commander", "group", NewWorkspace(Tmux, windows))
	if err != nil {
		t.Fatal(err)
	}
	windows[0].Command = "changed"
	if cell.Workspace.Windows[0].Command != "codex" {
		t.Fatalf("windows were shared: %#v", cell.Workspace.Windows)
	}
}

func TestMarkPendingとMarkReadyは同じ状態でも成功する(t *testing.T) {
	cell := newTestCommanderCell(t)
	cell.MarkReady()
	if cell.Status != Ready {
		t.Fatalf("status = %q", cell.Status)
	}
	cell.MarkPending()
	cell.MarkPending()
	if cell.Status != Pending {
		t.Fatalf("status = %q", cell.Status)
	}
	cell.MarkReady()
	if cell.Status != Ready {
		t.Fatalf("status = %q", cell.Status)
	}
}

func TestMarkDoneは未完了のCellだけを完了にする(t *testing.T) {
	cell := newTestCommanderCell(t)
	if err := cell.MarkDone(); err != nil {
		t.Fatal(err)
	}
	if !cell.Done {
		t.Fatal("cell was not marked done")
	}
	if err := cell.MarkDone(); err == nil {
		t.Fatal("marking a done cell done returned no error")
	}
	if !cell.Done {
		t.Fatal("a rejected MarkDone changed the state")
	}
}

func TestMarkUndoneは完了済みのCellだけを未完了に戻す(t *testing.T) {
	cell := newTestCommanderCell(t)
	if err := cell.MarkUndone(); err == nil {
		t.Fatal("marking an undone cell undone returned no error")
	}
	if cell.Done {
		t.Fatal("a rejected MarkUndone changed the state")
	}
	cell.Done = true
	if err := cell.MarkUndone(); err != nil {
		t.Fatal(err)
	}
	if cell.Done {
		t.Fatal("cell was not marked undone")
	}
}

func TestEnsureCanBeCleanedは完了済みのCellだけを許す(t *testing.T) {
	cell := newTestCommanderCell(t)
	if err := cell.EnsureCanBeCleaned(); err == nil {
		t.Fatal("an undone cell can be cleaned")
	}
	if err := cell.MarkDone(); err != nil {
		t.Fatal(err)
	}
	if err := cell.EnsureCanBeCleaned(); err != nil {
		t.Fatalf("a done cell cannot be cleaned: %v", err)
	}
}
