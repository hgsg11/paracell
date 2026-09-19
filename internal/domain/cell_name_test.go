package domain

import "testing"

func TestNewCellNameはIssueだけから名前を作る(t *testing.T) {
	if got := NewCellName("feature / 109"); got.Value != "feature-109" {
		t.Fatalf("cell name = %q", got.Value)
	}
}
