package usecase

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/hgsg11/paracell/internal/domain"
)

func TestEnterCellはCell名からWorkspaceへ入るコマンドを返す(t *testing.T) {
	ctx := context.Background()
	ports := newFakePorts()
	ports.cells = newUsecaseCellSet(t, [3]string{"cell-1", "123", "webapp"})
	_ = ports.cells.Groups[0].SetNote("検証中")

	cmd, err := (EnterCellUseCase{Cells: ports, WorkspaceFactory: ports}).Execute(ctx, EnterCellInput{Cell: "123"})
	if err != nil {
		t.Fatalf("EnterCellでエラーが返った: %v", err)
	}
	if cmd == nil || !reflect.DeepEqual(cmd.Args, []string{"tmux", "switch-client", "-t", "myapp-123"}) {
		t.Fatalf("cmd = %#v", cmd)
	}
	wantCalls := []string{"factory:workspace:tmux", "workspace:enter-command:123"}
	if !reflect.DeepEqual(ports.calls, wantCalls) {
		t.Fatalf("呼び出し順 = %#v, want %#v", ports.calls, wantCalls)
	}
	if ports.workspaceResource.DisplayLabel != "検証中" || ports.workspaceResource.WorkingDirectory != ".paracell/cells/123" {
		t.Fatalf("resource = %#v", ports.workspaceResource)
	}
}

func TestEnterCellはtmux以外のWorkspaceドライバを拒否する(t *testing.T) {
	ports := newFakePorts()
	ports.cells = newUsecaseCellSet(t, [3]string{"cell-1", "123", "webapp"})
	ports.cells.Commanders[0].Workspace = domain.NewWorkspace("screen", nil)
	_, err := (EnterCellUseCase{Cells: ports, WorkspaceFactory: ports}).Execute(context.Background(), EnterCellInput{Cell: "123"})
	if err == nil || !strings.Contains(err.Error(), `unsupported session driver "screen"`) {
		t.Fatalf("error = %v", err)
	}
	if len(ports.calls) != 0 {
		t.Fatalf("calls = %#v", ports.calls)
	}
}

func TestEnterCellは見つからないCellを伝える(t *testing.T) {
	ports := newFakePorts()
	ports.cells = newUsecaseCellSet(t, [3]string{"cell-1", "123", "webapp"})
	for _, identifier := range []string{"missing", "cell-1"} {
		if _, err := (EnterCellUseCase{Cells: ports, WorkspaceFactory: ports}).Execute(context.Background(), EnterCellInput{Cell: identifier}); err == nil || !strings.Contains(err.Error(), "not found") {
			t.Fatalf("Enter(%q) error = %v", identifier, err)
		}
	}
	if len(ports.calls) != 0 {
		t.Fatalf("calls = %#v", ports.calls)
	}
}
