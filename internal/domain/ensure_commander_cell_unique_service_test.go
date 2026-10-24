package domain

import (
	"strings"
	"testing"
)

func newServiceTestGroup(t *testing.T, id string, issue string) CellGroup {
	t.Helper()
	group, err := NewCellGroup(id, issue, "sample", "feat", Git, None, NoNotification)
	if err != nil {
		t.Fatal(err)
	}
	return group
}

func TestEnsureCommanderCellUniqueServiceは重複がなければ許す(t *testing.T) {
	groups := []CellGroup{newServiceTestGroup(t, "g1", "118"), newServiceTestGroup(t, "g2", "119")}
	if err := EnsureCommanderCellUniqueService(groups, "120", NewCellName("120")); err != nil {
		t.Fatal(err)
	}
	if err := EnsureCommanderCellUniqueService(nil, "120", NewCellName("120")); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureCommanderCellUniqueServiceはIssueの重複を拒否する(t *testing.T) {
	groups := []CellGroup{newServiceTestGroup(t, "g1", "118")}
	err := EnsureCommanderCellUniqueService(groups, "118", NewCellName("118"))
	if err == nil || !strings.Contains(err.Error(), `issue "118" already exists`) {
		t.Fatalf("error = %v", err)
	}
}

func TestEnsureCommanderCellUniqueServiceはCell名の重複を拒否する(t *testing.T) {
	groups := []CellGroup{newServiceTestGroup(t, "g1", "feat/118")}
	err := EnsureCommanderCellUniqueService(groups, "feat-118", NewCellName("feat-118"))
	if err == nil || !strings.Contains(err.Error(), `name "feat-118" already exists`) {
		t.Fatalf("error = %v", err)
	}
}
