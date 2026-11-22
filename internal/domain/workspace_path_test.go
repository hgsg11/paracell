package domain

import (
	"reflect"
	"testing"
)

func TestNewWorkspacePathはワークスペースからの相対パスに正規化する(t *testing.T) {
	cases := map[string]string{
		"internal/domain/queue.go":    "internal/domain/queue.go",
		"./README.md":                 "README.md",
		"internal//domain/../app.go":  "internal/app.go",
		`internal\adapter\state.go`:   "internal/adapter/state.go",
		"docs/日本語.md":                 "docs/日本語.md",
		"internal/./domain/./cell.go": "internal/domain/cell.go",
	}
	for input, want := range cases {
		got, err := NewWorkspacePath(input)
		if err != nil {
			t.Fatalf("NewWorkspacePath(%q): %v", input, err)
		}
		if got.Value != want {
			t.Fatalf("NewWorkspacePath(%q) = %q, want %q", input, got.Value, want)
		}
	}
}

func TestNewWorkspacePathは形式の誤ったパスを拒否する(t *testing.T) {
	for _, input := range []string{
		"", "  ", "/etc/passwd", `C:\work\a.go`, "../outside.go", "a/../../outside.go", "..", ".",
		"internal/", "internal/domain/", "*.go", "internal/**/a.go", "a?.go", "[ab].go", "{a,b}.go",
	} {
		if _, err := NewWorkspacePath(input); err == nil {
			t.Fatalf("NewWorkspacePath(%q) accepted", input)
		}
	}
}

func TestExistingPathsは与えたパスだけを含む(t *testing.T) {
	a, b := mustPath(t, "a.go"), mustPath(t, "b.go")
	existing := NewExistingPaths([]WorkspacePath{a})
	if !existing.Contains(a) || existing.Contains(b) {
		t.Fatalf("existing = %#v", existing)
	}
	if (ExistingPaths{}).Contains(a) {
		t.Fatal("zero ExistingPaths contains a path")
	}
}

func TestNewChangePlanは重複を除き両方にあるパスと空の宣言を拒否する(t *testing.T) {
	a, b, c := mustPath(t, "a.go"), mustPath(t, "b.go"), mustPath(t, "c.go")
	plan, err := NewChangePlan([]WorkspacePath{a, b, a}, []WorkspacePath{c, c})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(plan.Modify, []WorkspacePath{a, b}) || !reflect.DeepEqual(plan.Create, []WorkspacePath{c}) {
		t.Fatalf("plan = %#v", plan)
	}
	if !reflect.DeepEqual(plan.Paths(), []WorkspacePath{a, b, c}) {
		t.Fatalf("paths = %#v", plan.Paths())
	}
	if _, err := NewChangePlan([]WorkspacePath{a}, []WorkspacePath{a}); err == nil {
		t.Fatal("path in both lists accepted")
	}
	if _, err := NewChangePlan(nil, nil); err == nil {
		t.Fatal("empty plan accepted")
	}
}

func TestChangePlanOverlapsは種類に関係なく共通のパスで重なる(t *testing.T) {
	a, b, c := mustPath(t, "a.go"), mustPath(t, "b.go"), mustPath(t, "c.go")
	modifyA := mustPlan(t, []WorkspacePath{a}, nil)
	createA := mustPlan(t, nil, []WorkspacePath{a})
	other := mustPlan(t, []WorkspacePath{b}, []WorkspacePath{c})
	if !modifyA.Overlaps(createA) || !createA.Overlaps(modifyA) {
		t.Fatal("modify and create of the same path do not overlap")
	}
	if modifyA.Overlaps(other) || other.Overlaps(createA) {
		t.Fatal("disjoint plans overlap")
	}
}

func TestNewForkRequestはTemplateを必須にしNoteを正規化する(t *testing.T) {
	request, err := NewForkRequest("feat", "fix", "run", "  API\t実装 ")
	if err != nil {
		t.Fatal(err)
	}
	if request != (ForkRequest{Template: "feat", Prefix: "fix", Command: "run", Note: "API 実装"}) {
		t.Fatalf("request = %#v", request)
	}
	if _, err := NewForkRequest("", "", "", ""); err == nil {
		t.Fatal("empty template accepted")
	}
	if _, err := NewForkRequest("feat", "", "", "123456789012345678901"); err == nil {
		t.Fatal("long note accepted")
	}
}

func mustPath(t *testing.T, value string) WorkspacePath {
	t.Helper()
	path, err := NewWorkspacePath(value)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func mustPlan(t *testing.T, modify []WorkspacePath, create []WorkspacePath) ChangePlan {
	t.Helper()
	plan, err := NewChangePlan(modify, create)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}
