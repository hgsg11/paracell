package state

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hgsg11/paracell/internal/domain"
	"github.com/hgsg11/paracell/internal/usecase"
)

func TestSQLiteStateはReviewを保存しHandledUntilを進めCellの終了と同じ更新で消す(t *testing.T) {
	ctx := context.Background()
	adapter := newTestAdapter(t)
	group, commander := persistedTestCell(t, "group-1", "commander-1", "175")
	review, err := domain.NewReview("review-1", group.ID, []domain.PullRequestRef{{TargetCellID: "t-api", Number: "180"}, {TargetCellID: "t-web", Number: "12"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := adapter.UpdateCells(ctx, func(set usecase.CellSet) (usecase.CellSet, error) {
		set.Groups = append(set.Groups, group)
		set.Commanders = append(set.Commanders, commander)
		set.Reviews = append(set.Reviews, review)
		return set, nil
	}); err != nil {
		t.Fatal(err)
	}
	loaded, err := adapter.LoadCells(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Reviews) != 1 || !reflect.DeepEqual(loaded.Reviews[0].PullRequests, review.PullRequests) || !loaded.Reviews[0].HandledUntil.IsZero() {
		t.Fatalf("reviews = %#v", loaded.Reviews)
	}
	views, err := adapter.ListCellViews(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(views[0].PullRequests, []string{"180", "12"}) {
		t.Fatalf("view pull requests = %v", views[0].PullRequests)
	}

	at := time.Date(2026, 11, 23, 10, 0, 0, 123, time.FixedZone("JST", 9*60*60))
	if err := adapter.UpdateCells(ctx, func(set usecase.CellSet) (usecase.CellSet, error) {
		if err := set.Reviews[0].Acknowledge(at); err != nil {
			return usecase.CellSet{}, err
		}
		return set, nil
	}); err != nil {
		t.Fatal(err)
	}
	loaded, err = adapter.LoadCells(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.Reviews[0].HandledUntil.Equal(at) {
		t.Fatalf("handled until = %s, want %s", loaded.Reviews[0].HandledUntil, at)
	}

	if err := adapter.UpdateCells(ctx, func(set usecase.CellSet) (usecase.CellSet, error) {
		set.Reviews[0].PullRequests = []domain.PullRequestRef{{TargetCellID: "t-api", Number: "999"}}
		return set, nil
	}); err == nil || !strings.Contains(err.Error(), "cannot change") {
		t.Fatalf("changing pull requests: %v", err)
	}

	if err := adapter.UpdateCells(ctx, func(set usecase.CellSet) (usecase.CellSet, error) {
		if err := set.Commanders[0].MarkMerged(); err != nil {
			return usecase.CellSet{}, err
		}
		set.Reviews = nil
		return set, nil
	}); err != nil {
		t.Fatal(err)
	}
	loaded, err = adapter.LoadCells(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Reviews) != 0 || loaded.Commanders[0].Progress != domain.Merged {
		t.Fatalf("state = %#v", loaded)
	}
	views, err = adapter.ListCellViews(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(views[0].PullRequests) != 0 {
		t.Fatalf("view pull requests = %v", views[0].PullRequests)
	}
}

func TestSQLiteStateはCellGroupのないReviewと2つ目のReviewを拒否する(t *testing.T) {
	ctx := context.Background()
	adapter := newTestAdapter(t)
	group, commander := persistedTestCell(t, "group-1", "commander-1", "175")
	newReview := func(id string, groupID string) domain.Review {
		review, err := domain.NewReview(id, groupID, []domain.PullRequestRef{{TargetCellID: "t", Number: "1"}})
		if err != nil {
			t.Fatal(err)
		}
		return review
	}
	for name, reviews := range map[string][]domain.Review{
		"unknown group": {newReview("r1", "missing")},
		"two reviews":   {newReview("r1", group.ID), newReview("r2", group.ID)},
	} {
		err := adapter.UpdateCells(ctx, func(set usecase.CellSet) (usecase.CellSet, error) {
			set.Groups = append(set.Groups, group)
			set.Commanders = append(set.Commanders, commander)
			set.Reviews = reviews
			return set, nil
		})
		if err == nil {
			t.Fatalf("%s: no error", name)
		}
	}
}
