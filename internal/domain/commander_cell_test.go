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

func TestNewCommanderCellはReadyで実装中かつ版なしで作る(t *testing.T) {
	cell := newTestCommanderCell(t)
	if cell.ID != "commander" || cell.CellGroupID != "group" || cell.Status != Ready || cell.Progress != InProgress || cell.Version != 0 {
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

func TestCommanderCellの進み具合は決めた遷移だけを許す(t *testing.T) {
	marks := map[CellProgress]func(*CommanderCell) error{
		InProgress: (*CommanderCell).MarkInProgress,
		InReview:   (*CommanderCell).MarkInReview,
		Merged:     (*CommanderCell).MarkMerged,
		Abandoned:  (*CommanderCell).MarkAbandoned,
	}
	allowed := map[[2]CellProgress]bool{
		{InProgress, InReview}:  true,
		{InReview, InProgress}:  true,
		{InProgress, Merged}:    true,
		{InReview, Merged}:      true,
		{InProgress, Abandoned}: true,
		{InReview, Abandoned}:   true,
	}
	for _, from := range []CellProgress{InProgress, InReview, Merged, Abandoned} {
		for _, to := range []CellProgress{InProgress, InReview, Merged, Abandoned} {
			cell := newTestCommanderCell(t)
			cell.Progress = from
			err := marks[to](&cell)
			if allowed[[2]CellProgress{from, to}] {
				if err != nil || cell.Progress != to {
					t.Fatalf("%s -> %s: err = %v, progress = %s", from, to, err, cell.Progress)
				}
				continue
			}
			if err == nil || cell.Progress != from {
				t.Fatalf("%s -> %s: err = %v, progress = %s, want rejected", from, to, err, cell.Progress)
			}
		}
	}
}

func TestCellProgressはmerge済みと中止だけを終端にする(t *testing.T) {
	for progress, want := range map[CellProgress]bool{InProgress: false, InReview: false, Merged: true, Abandoned: true} {
		if progress.IsTerminal() != want {
			t.Fatalf("%s.IsTerminal() = %t", progress, !want)
		}
	}
}

func TestNewCellProgressは決めた値だけを受け付ける(t *testing.T) {
	for _, value := range []string{"in_progress", "in_review", "merged", "abandoned"} {
		if _, err := NewCellProgress(value); err != nil {
			t.Fatalf("NewCellProgress(%q): %v", value, err)
		}
	}
	for _, value := range []string{"", "done", "IN_PROGRESS"} {
		if _, err := NewCellProgress(value); err == nil {
			t.Fatalf("NewCellProgress(%q) returned no error", value)
		}
	}
}

func TestEnsureCanBeCleanedはmerge済みか中止のCellだけを許す(t *testing.T) {
	for progress, cleanable := range map[CellProgress]bool{InProgress: false, InReview: false, Merged: true, Abandoned: true} {
		cell := newTestCommanderCell(t)
		cell.Progress = progress
		if err := cell.EnsureCanBeCleaned(); (err == nil) != cleanable {
			t.Fatalf("%s: EnsureCanBeCleaned() = %v, want cleanable %t", progress, err, cleanable)
		}
	}
}
