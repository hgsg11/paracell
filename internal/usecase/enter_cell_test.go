package usecase

import (
	"context"
	"reflect"
	"testing"

	"github.com/shige1114/paradev/internal/domain"
)

func TestEnterCellはSessionにEnterを依頼する(t *testing.T) {
	ctx := context.Background()
	ports := newFakePorts()
	cell := domain.Cell{ID: "cell-1", Name: "123", Template: "webapp", Session: domain.Session{Name: "pdev-myapp-123"}}

	uc := EnterCellUseCase{
		Config:         ports,
		SessionFactory: ports,
	}

	got, err := uc.Execute(ctx, EnterCellInput{Cell: cell})
	if err != nil {
		t.Fatalf("EnterCellでエラーが返った: %v", err)
	}
	if !reflect.DeepEqual(got, cell) {
		t.Fatalf("cell = %#v, want %#v", got, cell)
	}
	wantCalls := []string{"factory:session:tmux", "session:enter:123"}
	if !reflect.DeepEqual(ports.calls, wantCalls) {
		t.Fatalf("呼び出し順 = %#v, want %#v", ports.calls, wantCalls)
	}
}
