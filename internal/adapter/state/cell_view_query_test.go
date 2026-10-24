package state

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/hgsg11/paracell/internal/domain"
	"github.com/hgsg11/paracell/internal/query"
)

func TestListCellViewsはCellがなければ空のスライスを返す(t *testing.T) {
	views, err := newTestAdapter(t).ListCellViews(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if views == nil || len(views) != 0 {
		t.Fatalf("views = %#v", views)
	}
}

func TestListCellViewsは3つの形式の行を一覧の順で読む(t *testing.T) {
	ctx := context.Background()
	adapter := newTestAdapter(t)
	group, commander := persistedTestCell(t, "group-id", "new", "feat/44 api")
	group.BeginCreation()
	if err := adapter.SaveCells(ctx, []domain.CellGroup{group}, []domain.CommanderCell{commander}); err != nil {
		t.Fatal(err)
	}
	insertRawRow(t, adapter, "cur", "43", 1, 2, currentGroupRecord)
	insertRawRow(t, adapter, "old", "42", 2, 3, legacyGroupRecord)
	views, err := adapter.ListCellViews(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := []query.CellView{
		{Name: "feat-44-api", Label: "feat-44-api", Template: "feat", Status: "ready", Done: false, CreationStatus: "creating"},
		{Name: "43", Label: "current note", Template: "feat", Status: "pending", Done: true, CreationStatus: "failed", FailedStage: "containers", LastError: "docker failed"},
		{Name: "42", Label: "old note", Template: "feat", Status: "pending", Done: true, CreationStatus: "ready"},
	}
	if !reflect.DeepEqual(views, want) {
		t.Fatalf("views = %#v, want %#v", views, want)
	}
}

func TestListCellViewsは作成に失敗したCellだけに失敗段階とエラーを入れる(t *testing.T) {
	ctx := context.Background()
	adapter := newTestAdapter(t)
	failedGroup, failed := persistedTestCell(t, "failed-group", "failed", "42")
	failedGroup.FailCreation(domain.CreationStageWorkspace, errors.New("tmux failed"))
	if err := failedGroup.SetNote("検証中"); err != nil {
		t.Fatal(err)
	}
	readyGroup, ready := persistedTestCell(t, "ready-group", "ready", "43")
	if err := adapter.SaveCells(ctx, []domain.CellGroup{failedGroup, readyGroup}, []domain.CommanderCell{failed, ready}); err != nil {
		t.Fatal(err)
	}
	views, err := adapter.ListCellViews(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(views) != 2 || views[0].FailedStage != "workspace" || views[0].LastError != "tmux failed" || views[0].Label != "検証中" || views[0].Name != "42" {
		t.Fatalf("failed view = %#v", views)
	}
	if views[1].FailedStage != "" || views[1].LastError != "" || views[1].Label != "43" || views[1].CreationStatus != "ready" {
		t.Fatalf("ready view = %#v", views[1])
	}
}

func TestListCellViewsは読めない行をエラーにする(t *testing.T) {
	adapter := newTestAdapter(t)
	insertRawRow(t, adapter, "broken", "42", 0, 1, `{"commander":`)
	if _, err := adapter.ListCellViews(context.Background()); err == nil || !strings.Contains(err.Error(), `"broken"`) {
		t.Fatalf("error = %v", err)
	}
	other := newTestAdapter(t)
	insertRawRow(t, other, "old", "42", 0, 9, legacyGroupRecord)
	if _, err := other.ListCellViews(context.Background()); err == nil {
		t.Fatal("record whose version differs from its row was listed")
	}
}
