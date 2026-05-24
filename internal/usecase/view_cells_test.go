package usecase

import (
	"context"
	"reflect"
	"testing"

	"github.com/shige1114/paradev/internal/domain"
)

func TestViewCellsはStateのCell一覧を返す(t *testing.T) {
	ctx := context.Background()
	ports := newFakePorts()
	ports.cells = []domain.Cell{
		{ID: "cell-1", Name: "123", Template: "default"},
		{ID: "cell-2", Name: "456", Template: "webapp"},
	}

	uc := ViewCellsUseCase{State: ports}
	cells, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("ViewCellsでエラーが返った: %v", err)
	}
	if !reflect.DeepEqual(cells, ports.cells) {
		t.Fatalf("cells = %#v, want %#v", cells, ports.cells)
	}
}
