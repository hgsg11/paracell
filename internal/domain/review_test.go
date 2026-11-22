package domain

import (
	"testing"
	"time"
)

func TestNewReviewはPRが1つ以上でTargetが重ならないことを確かめる(t *testing.T) {
	refs := []PullRequestRef{{TargetCellID: "t1", Number: "180"}, {TargetCellID: "t2", Number: "12"}}
	review, err := NewReview("r1", "g1", refs)
	if err != nil {
		t.Fatal(err)
	}
	if review.ID != "r1" || review.CellGroupID != "g1" || len(review.PullRequests) != 2 || !review.HandledUntil.IsZero() {
		t.Fatalf("review = %#v", review)
	}
	refs[0].Number = "999"
	if review.PullRequests[0].Number != "180" {
		t.Fatal("review shares the caller's slice")
	}
	for name, input := range map[string][]PullRequestRef{
		"no pull request":  nil,
		"duplicate target": {{TargetCellID: "t1", Number: "1"}, {TargetCellID: "t1", Number: "2"}},
		"empty number":     {{TargetCellID: "t1"}},
		"empty target":     {{Number: "1"}},
	} {
		if _, err := NewReview("r1", "g1", input); err == nil {
			t.Fatalf("%s: no error", name)
		}
	}
	if _, err := NewReview("", "g1", []PullRequestRef{{TargetCellID: "t1", Number: "1"}}); err == nil {
		t.Fatal("empty id: no error")
	}
	if _, err := NewReview("r1", "", []PullRequestRef{{TargetCellID: "t1", Number: "1"}}); err == nil {
		t.Fatal("empty CellGroup: no error")
	}
}

func TestReviewのAcknowledgeはHandledUntilを進めるだけで戻さない(t *testing.T) {
	review, err := NewReview("r1", "g1", []PullRequestRef{{TargetCellID: "t1", Number: "1"}})
	if err != nil {
		t.Fatal(err)
	}
	later := time.Date(2026, 11, 23, 10, 0, 0, 0, time.UTC)
	if err := review.Acknowledge(later); err != nil {
		t.Fatal(err)
	}
	if err := review.Acknowledge(later); err != nil {
		t.Fatalf("same time: %v", err)
	}
	if err := review.Acknowledge(later.Add(-time.Second)); err == nil {
		t.Fatal("moving back returned no error")
	}
	if !review.HandledUntil.Equal(later) {
		t.Fatalf("handled until = %s", review.HandledUntil)
	}
}

func TestReviewFactoryは保存された値を検証して復元する(t *testing.T) {
	at := time.Date(2026, 11, 23, 10, 0, 0, 0, time.UTC)
	review, err := ReviewFactory{}.Restore("r1", "g1", []RestoredPullRequestRef{{TargetCellID: "t1", Number: "7"}}, at)
	if err != nil {
		t.Fatal(err)
	}
	if !review.HandledUntil.Equal(at) || review.PullRequests[0] != (PullRequestRef{TargetCellID: "t1", Number: "7"}) {
		t.Fatalf("review = %#v", review)
	}
	if _, err := (ReviewFactory{}).Restore("r1", "g1", nil, at); err == nil {
		t.Fatal("restoring a review without pull requests returned no error")
	}
}

func TestDecideReviewOutcomeはPRの状態の集まりから結果を決める(t *testing.T) {
	open := PullRequestStatus{State: PullRequestOpen}
	merged := PullRequestStatus{State: PullRequestMerged}
	closed := PullRequestStatus{State: PullRequestClosed}
	for name, tc := range map[string]struct {
		statuses []PullRequestStatus
		want     ReviewOutcome
	}{
		"all open":          {[]PullRequestStatus{open, open}, ReviewOpen},
		"open and merged":   {[]PullRequestStatus{merged, open}, ReviewOpen},
		"open and closed":   {[]PullRequestStatus{closed, open}, ReviewOpen},
		"all merged":        {[]PullRequestStatus{merged, merged}, ReviewMerged},
		"all closed":        {[]PullRequestStatus{closed}, ReviewClosed},
		"merged and closed": {[]PullRequestStatus{merged, closed}, ReviewMixed},
		"none":              {nil, ReviewOpen},
	} {
		if got := DecideReviewOutcome(tc.statuses); got != tc.want {
			t.Fatalf("%s: outcome = %s, want %s", name, got, tc.want)
		}
	}
}

func TestIsReviewApprovedは開いているPRがすべてApproveされたときだけ真(t *testing.T) {
	approved := PullRequestStatus{State: PullRequestOpen, Approval: ApprovalApproved}
	changes := PullRequestStatus{State: PullRequestOpen, Approval: ApprovalChangesRequested}
	unreviewed := PullRequestStatus{State: PullRequestOpen, Approval: ApprovalNone}
	merged := PullRequestStatus{State: PullRequestMerged}
	closed := PullRequestStatus{State: PullRequestClosed}
	for name, tc := range map[string]struct {
		statuses []PullRequestStatus
		want     bool
	}{
		"all approved":             {[]PullRequestStatus{approved, approved}, true},
		"approved and merged":      {[]PullRequestStatus{merged, approved}, true},
		"one changes requested":    {[]PullRequestStatus{approved, changes}, false},
		"one without review":       {[]PullRequestStatus{approved, unreviewed}, false},
		"approved and closed":      {[]PullRequestStatus{approved, closed}, false},
		"all merged, nothing open": {[]PullRequestStatus{merged}, false},
	} {
		if got := IsReviewApproved(tc.statuses); got != tc.want {
			t.Fatalf("%s: approved = %v, want %v", name, got, tc.want)
		}
	}
}

func TestNewReviewDriverTypeは省略をnoneにして不明な値を拒否する(t *testing.T) {
	for value, want := range map[string]ReviewDriverType{"": NoReview, "none": NoReview, "github": GitHubReview} {
		got, err := NewReviewDriverType(value)
		if err != nil || got != want {
			t.Fatalf("%q: driver = %q, err = %v", value, got, err)
		}
	}
	if _, err := NewReviewDriverType("gitlab"); err == nil {
		t.Fatal("unknown driver returned no error")
	}
	if NoReview.Enabled() || ReviewDriverType("").Enabled() || !GitHubReview.Enabled() {
		t.Fatal("Enabled is wrong")
	}
}
