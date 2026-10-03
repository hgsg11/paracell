package usecase

import (
	"context"
	"reflect"
	"testing"
)

func TestEnterCellはWorkspaceにEnterを依頼する(t *testing.T) {
	ctx := context.Background()
	ports := newFakePorts()
	cell := newUsecaseTestCell(t, "cell-1", "123", "webapp")

	uc := EnterCellUseCase{WorkspaceFactory: ports}

	got, err := uc.Execute(ctx, EnterCellInput{Cell: cell})
	if err != nil {
		t.Fatalf("EnterCellでエラーが返った: %v", err)
	}
	if !reflect.DeepEqual(got, cell) {
		t.Fatalf("cell = %#v, want %#v", got, cell)
	}
	wantCalls := []string{"factory:workspace:tmux", "workspace:enter:123"}
	if !reflect.DeepEqual(ports.calls, wantCalls) {
		t.Fatalf("呼び出し順 = %#v, want %#v", ports.calls, wantCalls)
	}
}
