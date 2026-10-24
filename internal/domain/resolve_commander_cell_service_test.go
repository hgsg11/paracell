package domain

import (
	"reflect"
	"testing"
)

func newServiceTestCommander(t *testing.T, id string, groupID string) CommanderCell {
	t.Helper()
	cell, err := NewCommanderCell(id, groupID, NewWorkspace(Tmux, nil))
	if err != nil {
		t.Fatal(err)
	}
	return cell
}

func TestResolveCommanderCellServiceはCellGroupのIDとIssueとCell名で探す(t *testing.T) {
	groups := []CellGroup{newServiceTestGroup(t, "g1", "118"), newServiceTestGroup(t, "g2", "feat/119")}
	commanders := []CommanderCell{newServiceTestCommander(t, "c1", "g1"), newServiceTestCommander(t, "c2", "g2")}
	for _, identifier := range []string{"g2", "feat/119", "feat-119"} {
		got, ok := ResolveCommanderCellService(groups, commanders, identifier)
		if !ok || !reflect.DeepEqual(got, commanders[1]) {
			t.Fatalf("Resolve(%q) = %#v, %t", identifier, got, ok)
		}
	}
}

func TestResolveCommanderCellServiceはCommanderCellのIDでは探さない(t *testing.T) {
	groups := []CellGroup{newServiceTestGroup(t, "g1", "118")}
	commanders := []CommanderCell{newServiceTestCommander(t, "c1", "g1")}
	if got, ok := ResolveCommanderCellService(groups, commanders, "c1"); ok {
		t.Fatalf("Resolve(c1) = %#v", got)
	}
	if got, ok := ResolveCommanderCellService(groups, commanders, ""); ok {
		t.Fatalf("Resolve(\"\") = %#v", got)
	}
}

func TestResolveCommanderCellServiceは一致するCellGroupがなければ見つからない(t *testing.T) {
	groups := []CellGroup{newServiceTestGroup(t, "g1", "118")}
	commanders := []CommanderCell{newServiceTestCommander(t, "c1", "g1")}
	if got, ok := ResolveCommanderCellService(groups, commanders, "missing"); ok || !reflect.DeepEqual(got, CommanderCell{}) {
		t.Fatalf("Resolve(missing) = %#v, %t", got, ok)
	}
	if _, ok := ResolveCommanderCellService(nil, nil, "118"); ok {
		t.Fatal("resolved from empty lists")
	}
}

func TestResolveCommanderCellServiceはCommanderCellのないCellGroupなら見つからない(t *testing.T) {
	groups := []CellGroup{newServiceTestGroup(t, "g1", "118"), newServiceTestGroup(t, "g2", "119")}
	commanders := []CommanderCell{newServiceTestCommander(t, "c2", "g2")}
	if got, ok := ResolveCommanderCellService(groups, commanders, "118"); ok || !reflect.DeepEqual(got, CommanderCell{}) {
		t.Fatalf("Resolve(118) = %#v, %t", got, ok)
	}
}

func TestResolveCommanderCellServiceは複数一致なら一覧の順で最初のCellGroupを選ぶ(t *testing.T) {
	// The issue of g2 equals the Cell name of g1, so "a-b" matches both groups.
	groups := []CellGroup{newServiceTestGroup(t, "g1", "a/b"), newServiceTestGroup(t, "g2", "a-b")}
	commanders := []CommanderCell{newServiceTestCommander(t, "c2", "g2"), newServiceTestCommander(t, "c1", "g1")}
	got, ok := ResolveCommanderCellService(groups, commanders, "a-b")
	if !ok || got.ID != "c1" {
		t.Fatalf("Resolve(a-b) = %#v, %t", got, ok)
	}
	reversed := []CellGroup{groups[1], groups[0]}
	got, ok = ResolveCommanderCellService(reversed, commanders, "a-b")
	if !ok || got.ID != "c2" {
		t.Fatalf("Resolve(a-b) with reversed groups = %#v, %t", got, ok)
	}
}
