package usecase

import (
	"context"
	"reflect"
	"testing"
)

func TestEnterRootSessionはProject名を使ってSessionに委譲する(t *testing.T) {
	ctx := context.Background()
	ports := newFakePorts()

	uc := EnterRootSessionUseCase{
		Config:         ports,
		SessionFactory: ports,
	}

	if err := uc.Execute(ctx); err != nil {
		t.Fatalf("Executeでエラーが返った: %v", err)
	}
	wantCalls := []string{"factory:session:tmux", "session:enter-root:myapp"}
	if !reflect.DeepEqual(ports.calls, wantCalls) {
		t.Fatalf("呼び出し順 = %#v, want %#v", ports.calls, wantCalls)
	}
}
