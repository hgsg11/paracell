package domain

import (
	"reflect"
	"testing"
)

func TestCommanderCellFactoryRestoreは保存値からCellを戻す(t *testing.T) {
	windows := []RestoredWorkspaceWindow{{Name: "agent", Command: "codex implement"}, {Name: "shell", Command: ""}}
	cell, err := CommanderCellFactory{}.Restore("commander", "group", "tmux", windows, "pending", "merged", 7)
	if err != nil {
		t.Fatal(err)
	}
	want := CommanderCell{
		ID: "commander", CellGroupID: "group",
		Workspace: NewWorkspace(Tmux, []WorkspaceWindow{{Name: "agent", Command: "codex implement"}, {Name: "shell"}}),
		Status:    Pending, Progress: Merged, Version: 7,
	}
	if !reflect.DeepEqual(cell, want) {
		t.Fatalf("cell = %#v, want %#v", cell, want)
	}
}

func TestCommanderCellFactoryRestoreは実装中とReadyも戻す(t *testing.T) {
	cell, err := CommanderCellFactory{}.Restore("commander", "group", "tmux", nil, "ready", "in_progress", 1)
	if err != nil {
		t.Fatal(err)
	}
	if cell.Progress != InProgress || cell.Status != Ready || cell.Version != 1 || len(cell.Workspace.Windows) != 0 {
		t.Fatalf("cell = %#v", cell)
	}
}

func TestCommanderCellFactoryRestoreは不正な値を拒否する(t *testing.T) {
	cases := map[string]func() error{
		"workspace driver": func() error {
			_, err := CommanderCellFactory{}.Restore("commander", "group", "screen", nil, "ready", "in_progress", 1)
			return err
		},
		"window": func() error {
			_, err := CommanderCellFactory{}.Restore("commander", "group", "tmux", []RestoredWorkspaceWindow{{Name: "", Command: "codex"}}, "ready", "in_progress", 1)
			return err
		},
		"status": func() error {
			_, err := CommanderCellFactory{}.Restore("commander", "group", "tmux", nil, "running", "in_progress", 1)
			return err
		},
		"progress": func() error {
			_, err := CommanderCellFactory{}.Restore("commander", "group", "tmux", nil, "ready", "done", 1)
			return err
		},
		"empty progress": func() error {
			_, err := CommanderCellFactory{}.Restore("commander", "group", "tmux", nil, "ready", "", 1)
			return err
		},
		"version 0": func() error {
			_, err := CommanderCellFactory{}.Restore("commander", "group", "tmux", nil, "ready", "in_progress", 0)
			return err
		},
		"id": func() error {
			_, err := CommanderCellFactory{}.Restore("", "group", "tmux", nil, "ready", "in_progress", 1)
			return err
		},
		"CellGroup reference": func() error {
			_, err := CommanderCellFactory{}.Restore("commander", "", "tmux", nil, "ready", "in_progress", 1)
			return err
		},
	}
	for name, restore := range cases {
		t.Run(name, func(t *testing.T) {
			if err := restore(); err == nil {
				t.Fatal("invalid value accepted")
			}
		})
	}
}
