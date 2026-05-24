package session

import (
	"context"
	"reflect"
	"testing"

	"github.com/shige1114/paradev/internal/domain"
)

type fakeRunner struct {
	calls []string
}

func (r *fakeRunner) Run(ctx context.Context, name string, args ...string) error {
	_ = ctx
	r.calls = append(r.calls, name+" "+joinArgs(args))
	return nil
}

func (r *fakeRunner) Output(ctx context.Context, name string, args ...string) (string, error) {
	_ = ctx
	_ = name
	_ = args
	return "", nil
}

func joinArgs(args []string) string {
	if len(args) == 0 {
		return ""
	}
	out := args[0]
	for _, arg := range args[1:] {
		out += " " + arg
	}
	return out
}

func TestEnterSessionはTMUX外ならattachSessionを使う(t *testing.T) {
	t.Setenv("TMUX", "")
	runner := &fakeRunner{}
	adapter := TmuxAdapter{Runner: runner}
	cell := domain.Cell{Session: domain.Session{Name: "paracell-myapp-123"}}

	if err := adapter.EnterSession(context.Background(), cell); err != nil {
		t.Fatalf("EnterSessionでエラーが返った: %v", err)
	}
	want := []string{"tmux attach-session -t paracell-myapp-123"}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestEnterSessionはTMUX内ならswitchClientを使う(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-1000/default,123,0")
	runner := &fakeRunner{}
	adapter := TmuxAdapter{Runner: runner}
	cell := domain.Cell{Session: domain.Session{Name: "paracell-myapp-123"}}

	if err := adapter.EnterSession(context.Background(), cell); err != nil {
		t.Fatalf("EnterSessionでエラーが返った: %v", err)
	}
	want := []string{"tmux switch-client -t paracell-myapp-123"}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestCreateSessionはWindow未指定ならSessionだけ作る(t *testing.T) {
	runner := &fakeRunner{}
	adapter := TmuxAdapter{Runner: runner}
	cell := domain.Cell{
		Source:  domain.Source{Path: ".paracell/cells/123/source"},
		Session: domain.Session{Name: "paracell-myapp-123"},
	}

	if err := adapter.CreateSession(context.Background(), cell); err != nil {
		t.Fatalf("CreateSessionでエラーが返った: %v", err)
	}
	want := []string{"tmux new-session -d -s paracell-myapp-123 -c .paracell/cells/123/source"}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestCreateSessionは指定Windowを作る(t *testing.T) {
	runner := &fakeRunner{}
	adapter := TmuxAdapter{Runner: runner}
	cell := domain.Cell{
		Source: domain.Source{Path: ".paracell/cells/123/source"},
		Session: domain.Session{
			Name: "paracell-myapp-123",
			Windows: []domain.SessionWindow{
				{Name: "editor"},
				{Name: "server"},
			},
		},
	}

	if err := adapter.CreateSession(context.Background(), cell); err != nil {
		t.Fatalf("CreateSessionでエラーが返った: %v", err)
	}
	want := []string{
		"tmux new-session -d -s paracell-myapp-123 -n editor -c .paracell/cells/123/source",
		"tmux new-window -t paracell-myapp-123 -n server -c .paracell/cells/123/source",
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestCreateSessionはWindow作成後にCommandをEnterで実行する(t *testing.T) {
	runner := &fakeRunner{}
	adapter := TmuxAdapter{Runner: runner}
	cell := domain.Cell{
		Source: domain.Source{Path: ".paracell/cells/123/source"},
		Session: domain.Session{
			Name: "paracell-myapp-123",
			Windows: []domain.SessionWindow{
				{Name: "editor", Command: "nvim ."},
				{Name: "server"},
				{Name: "test", Command: "go test ./..."},
			},
		},
	}

	if err := adapter.CreateSession(context.Background(), cell); err != nil {
		t.Fatalf("CreateSessionでエラーが返った: %v", err)
	}
	want := []string{
		"tmux new-session -d -s paracell-myapp-123 -n editor -c .paracell/cells/123/source",
		"tmux send-keys -t paracell-myapp-123:editor nvim . Enter",
		"tmux new-window -t paracell-myapp-123 -n server -c .paracell/cells/123/source",
		"tmux new-window -t paracell-myapp-123 -n test -c .paracell/cells/123/source",
		"tmux send-keys -t paracell-myapp-123:test go test ./... Enter",
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}
