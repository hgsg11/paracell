package usecase

import (
	"context"
	"reflect"
	"testing"
)

func TestExitSessionはSessionに委譲する(t *testing.T) {
	ctx := context.Background()
	ports := newFakePorts()
	uc := ExitSessionUseCase{
		Config:         ports,
		SessionFactory: ports,
	}

	if err := uc.Execute(ctx); err != nil {
		t.Fatalf("Executeでエラーが返った: %v", err)
	}
	wantCalls := []string{"factory:session:tmux", "session:exit"}
	if !reflect.DeepEqual(ports.calls, wantCalls) {
		t.Fatalf("呼び出し順 = %#v, want %#v", ports.calls, wantCalls)
	}
}
