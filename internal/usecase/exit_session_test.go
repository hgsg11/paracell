package usecase

import (
	"context"
	"reflect"
	"testing"
)

func TestExitWorkspaceはWorkspaceに委譲する(t *testing.T) {
	ctx := context.Background()
	ports := newFakePorts()
	uc := ExitWorkspaceUseCase{
		Config:           ports,
		WorkspaceFactory: ports,
	}

	if err := uc.Execute(ctx); err != nil {
		t.Fatalf("Executeでエラーが返った: %v", err)
	}
	wantCalls := []string{"factory:workspace:tmux", "workspace:exit"}
	if !reflect.DeepEqual(ports.calls, wantCalls) {
		t.Fatalf("呼び出し順 = %#v, want %#v", ports.calls, wantCalls)
	}
}
