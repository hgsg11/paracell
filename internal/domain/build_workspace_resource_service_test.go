package domain

import (
	"path/filepath"
	"reflect"
	"testing"
)

func newWorkspaceServiceTestTarget(t *testing.T, name string, withSource bool) TargetCell {
	t.Helper()
	var source *Source
	var containers []*Container
	if withSource {
		value, err := NewSource(".", "main", "feat/120")
		if err != nil {
			t.Fatal(err)
		}
		source = &value
	} else {
		container, err := NewContainer(nil, "db", Target)
		if err != nil {
			t.Fatal(err)
		}
		containers = []*Container{&container}
	}
	target, err := NewTargetCell(name, "g1", name, source, containers)
	if err != nil {
		t.Fatal(err)
	}
	return target
}

func newWorkspaceServiceTestWorkspace(t *testing.T) Workspace {
	t.Helper()
	window, err := NewWorkspaceWindow("claude", "claude")
	if err != nil {
		t.Fatal(err)
	}
	return NewWorkspace(Tmux, []WorkspaceWindow{window})
}

func TestBuildWorkspaceResourceServiceはsourceを持つTargetがあればCellのディレクトリで起動する(t *testing.T) {
	group := newServiceTestGroup(t, "g1", "120")
	workspace := newWorkspaceServiceTestWorkspace(t)
	targets := []TargetCell{newWorkspaceServiceTestTarget(t, "db", false), newWorkspaceServiceTestTarget(t, "api", true)}
	got := BuildWorkspaceResourceService(group, workspace, targets)
	if want := group.WorkspaceResource(workspace); !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildWorkspaceResourceService() = %#v, want %#v", got, want)
	}
	if want := filepath.Join(".paracell", "cells", "120"); got.WorkingDirectory != want {
		t.Fatalf("WorkingDirectory = %q, want %q", got.WorkingDirectory, want)
	}
}

func TestBuildWorkspaceResourceServiceはsourceを持つTargetがなければプロジェクトのルートで起動する(t *testing.T) {
	group := newServiceTestGroup(t, "g1", "120")
	workspace := newWorkspaceServiceTestWorkspace(t)
	base := group.WorkspaceResource(workspace)
	for _, targets := range [][]TargetCell{nil, {newWorkspaceServiceTestTarget(t, "db", false)}} {
		got := BuildWorkspaceResourceService(group, workspace, targets)
		want := NewWorkspaceResource(base.Name, base.CellName, base.Project, base.DisplayLabel, ".", base.Windows)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("BuildWorkspaceResourceService(%d targets) = %#v, want %#v", len(targets), got, want)
		}
	}
}
