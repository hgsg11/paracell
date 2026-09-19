package domain

import (
	"strings"
	"testing"
)

func testCell(t *testing.T) Cell {
	t.Helper()
	source, err := NewSource(".", "main", "feat/42")
	if err != nil {
		t.Fatal(err)
	}
	sessionDriver, _ := NewSessionDriverType("tmux")
	sourceDriver, _ := NewSourceDriverType("git")
	cell, err := NewCell("id", "42", "sample", "feat", NewSources(sourceDriver, []Source{source}), NewContainers(None, nil), NewSession(sessionDriver, nil), NoNotification)
	if err != nil {
		t.Fatal(err)
	}
	return cell
}

func TestCellNoteは空白を正規化してUnicode文字数で検証する(t *testing.T) {
	tests := []struct {
		input, want string
		wantErr     bool
	}{
		{input: "  API\t実装\n  中  ", want: "API 実装 中"},
		{input: "案", want: "案"},
		{input: strings.Repeat("案", 20), want: strings.Repeat("案", 20)},
		{input: " \t\n ", wantErr: true},
		{input: strings.Repeat("案", 21), wantErr: true},
	}
	for _, tt := range tests {
		got, err := NormalizeCellNote(tt.input)
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Fatalf("NormalizeCellNote(%q) = %q, %v", tt.input, got, err)
		}
	}
}

func TestCellのResource名は保存せずIdentityから導出する(t *testing.T) {
	cell := testCell(t)
	if cell.Name().Value != "42" {
		t.Fatalf("name = %q", cell.Name())
	}
	if cell.SourceWorktreePath(cell.Sources.Items[0]) != ".paracell/cells/42/source" {
		t.Fatalf("path = %q", cell.SourceWorktreePath(cell.Sources.Items[0]))
	}
	if cell.ContainerNetworkName() != "paracell-sample-42" {
		t.Fatalf("network = %q", cell.ContainerNetworkName())
	}
	if cell.SessionName() != "sample-42" {
		t.Fatalf("session = %q", cell.SessionName())
	}
}

func TestCellはProjectIdentityを保持してResource名だけを正規化する(t *testing.T) {
	cell := testCell(t)
	cell.Project = "My App"
	if cell.Stored().Project != "My App" {
		t.Fatalf("project = %q", cell.Stored().Project)
	}
	if cell.ContainerNetworkName() != "paracell-My-App-42" || cell.SessionName() != "My-App-42" {
		t.Fatalf("network = %q, session = %q", cell.ContainerNetworkName(), cell.SessionName())
	}
}

func TestCell状態変更関数はAggregateを更新する(t *testing.T) {
	cell := testCell(t)
	err := cell.SetNote("実装中")
	if err != nil || cell.DisplayLabel() != "実装中" {
		t.Fatalf("note = %#v, %v", cell, err)
	}
	cell.ToggleDone()
	if err := cell.EnsureCanBeCleaned(); err != nil {
		t.Fatal(err)
	}
	err = cell.SetStatus(Pending)
	if err != nil || cell.Summary().Status != Pending {
		t.Fatalf("status = %#v, %v", cell, err)
	}
}

func Test新規CellのVersionは1でPersistence成功時だけ進む(t *testing.T) {
	cell := testCell(t)
	if cell.Version != 1 {
		t.Fatalf("version = %d", cell.Version)
	}
	persisted := cell
	if err := persisted.AdvanceVersion(); err != nil {
		t.Fatal(err)
	}
	if persisted.Version != 2 || cell.Version != 1 {
		t.Fatalf("versions = %d, %d", persisted.Version, cell.Version)
	}
}
