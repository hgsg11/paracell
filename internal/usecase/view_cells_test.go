package usecase

import (
	"context"
	"reflect"
	"testing"

	"github.com/hgsg11/paracell/internal/domain"
)

func TestViewCellsはStateのCell一覧を返す(t *testing.T) {
	ctx := context.Background()
	ports := newFakePorts()
	ports.cells = []domain.Cell{
		newUsecaseTestCell(t, "cell-1", "123", "default"),
		newUsecaseTestCell(t, "cell-2", "456", "webapp"),
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
