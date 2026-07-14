package session

import (
	"context"
	"errors"
	"os/exec"
	"reflect"
	"testing"

	"github.com/hgsg11/paracell/internal/domain"
)

type fakeRunner struct {
	calls  []string
	errors map[string]error
}

func (r *fakeRunner) Run(ctx context.Context, name string, args ...string) error {
	_ = ctx
	call := name + " " + joinArgs(args)
	r.calls = append(r.calls, call)
	if r.errors != nil && r.errors[call] != nil {
		return r.errors[call]
	}
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

func TestEnterRootSessionはSessionがなければ作成してAttachする(t *testing.T) {
	t.Setenv("TMUX", "")
	runner := &fakeRunner{
		errors: map[string]error{
			"tmux has-session -t paracell-myapp-root": errors.New("exit status 1: can't find session: paracell-myapp-root"),
		},
	}
	adapter := TmuxAdapter{Runner: runner}

	if err := adapter.EnterRootSession(context.Background(), domain.ProjectConfig{Name: "myapp"}); err != nil {
		t.Fatalf("EnterRootSessionでエラーが返った: %v", err)
	}
	want := []string{
		"tmux has-session -t paracell-myapp-root",
		"tmux new-session -d -s paracell-myapp-root -c .",
		"tmux set-option -t paracell-myapp-root key-table paracell",
		"tmux bind-key -T paracell C-t next-window",
		"tmux bind-key -T paracell C-p display-popup -E paracell view",
		"tmux attach-session -t paracell-myapp-root",
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestEnterRootSessionはPopup起動用にProjectRootを引き回す(t *testing.T) {
	t.Setenv("TMUX", "")
	runner := &fakeRunner{
		errors: map[string]error{
			"tmux has-session -t paracell-myapp-root": errors.New("exit status 1: can't find session: paracell-myapp-root"),
		},
	}
	adapter := TmuxAdapter{Runner: runner, Root: "/project"}

	if err := adapter.EnterRootSession(context.Background(), domain.ProjectConfig{Name: "myapp"}); err != nil {
		t.Fatalf("EnterRootSessionでエラーが返った: %v", err)
	}
	want := []string{
		"tmux has-session -t paracell-myapp-root",
		"tmux new-session -d -s paracell-myapp-root -e PARACELL_ROOT=/project -c /project",
		"tmux set-option -t paracell-myapp-root key-table paracell",
		"tmux bind-key -T paracell C-t next-window",
		"tmux bind-key -T paracell C-p display-popup -d /project -E env PARACELL_ROOT=/project paracell view",
		"tmux attach-session -t paracell-myapp-root",
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestEnterRootSessionはHasSessionがexitStatus1だけでも作成してAttachする(t *testing.T) {
	t.Setenv("TMUX", "")
	runner := &fakeRunner{
		errors: map[string]error{
			"tmux has-session -t paracell-myapp-root": &exec.ExitError{},
		},
	}
	adapter := TmuxAdapter{Runner: runner}

	if err := adapter.EnterRootSession(context.Background(), domain.ProjectConfig{Name: "myapp"}); err != nil {
		t.Fatalf("EnterRootSessionでエラーが返った: %v", err)
	}
	want := []string{
		"tmux has-session -t paracell-myapp-root",
		"tmux new-session -d -s paracell-myapp-root -c .",
		"tmux set-option -t paracell-myapp-root key-table paracell",
		"tmux bind-key -T paracell C-t next-window",
		"tmux bind-key -T paracell C-p display-popup -E paracell view",
		"tmux attach-session -t paracell-myapp-root",
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestEnterRootSessionは既存SessionでもPopupBindingを更新する(t *testing.T) {
	t.Setenv("TMUX", "")
	runner := &fakeRunner{}
	adapter := TmuxAdapter{Runner: runner, Root: "/project"}

	if err := adapter.EnterRootSession(context.Background(), domain.ProjectConfig{Name: "myapp"}); err != nil {
		t.Fatalf("EnterRootSessionでエラーが返った: %v", err)
	}
	want := []string{
		"tmux has-session -t paracell-myapp-root",
		"tmux set-option -t paracell-myapp-root key-table paracell",
		"tmux bind-key -T paracell C-t next-window",
		"tmux bind-key -T paracell C-p display-popup -d /project -E env PARACELL_ROOT=/project paracell view",
		"tmux attach-session -t paracell-myapp-root",
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestCreateSessionはWindow未指定ならSessionだけ作る(t *testing.T) {
	runner := &fakeRunner{}
	adapter := TmuxAdapter{Runner: runner, Root: "/project"}
	cell := domain.Cell{
		Issue:   "123",
		Name:    "123",
		Source:  domain.Source{Path: ".paracell/cells/123/source"},
		Session: domain.Session{Name: "paracell-myapp-123"},
	}

	if err := adapter.CreateSession(context.Background(), cell); err != nil {
		t.Fatalf("CreateSessionでエラーが返った: %v", err)
	}
	want := []string{
		"tmux new-session -d -s paracell-myapp-123 -e PARACELL_CELL=123 -e PARACELL_ROOT=/project -c .paracell/cells/123/source",
		"tmux set-option -t paracell-myapp-123 key-table paracell",
		"tmux bind-key -T paracell C-t next-window",
		"tmux bind-key -T paracell C-p display-popup -d /project -E env PARACELL_ROOT=/project paracell view",
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestCreateSessionは指定Windowを作る(t *testing.T) {
	runner := &fakeRunner{}
	adapter := TmuxAdapter{Runner: runner, Root: "/project"}
	cell := domain.Cell{
		Issue:  "123",
		Name:   "123",
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
		"tmux new-session -d -s paracell-myapp-123 -e PARACELL_CELL=123 -e PARACELL_ROOT=/project -n editor -c .paracell/cells/123/source",
		"tmux new-window -t paracell-myapp-123 -n server -c .paracell/cells/123/source",
		"tmux set-option -t paracell-myapp-123 key-table paracell",
		"tmux bind-key -T paracell C-t next-window",
		"tmux bind-key -T paracell C-p display-popup -d /project -E env PARACELL_ROOT=/project paracell view",
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestCreateSessionはWindow作成後にCommandをEnterで実行する(t *testing.T) {
	runner := &fakeRunner{}
	adapter := TmuxAdapter{Runner: runner, Root: "/project"}
	cell := domain.Cell{
		Issue:  "123",
		Name:   "123",
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
		"tmux new-session -d -s paracell-myapp-123 -e PARACELL_CELL=123 -e PARACELL_ROOT=/project -n editor -c .paracell/cells/123/source",
		"tmux send-keys -t paracell-myapp-123:editor nvim . Enter",
		"tmux new-window -t paracell-myapp-123 -n server -c .paracell/cells/123/source",
		"tmux new-window -t paracell-myapp-123 -n test -c .paracell/cells/123/source",
		"tmux send-keys -t paracell-myapp-123:test go test ./... Enter",
		"tmux set-option -t paracell-myapp-123 key-table paracell",
		"tmux bind-key -T paracell C-t next-window",
		"tmux bind-key -T paracell C-p display-popup -d /project -E env PARACELL_ROOT=/project paracell view",
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestCleanSessionは見つからないSessionをnotFound扱いにする(t *testing.T) {
	runner := &fakeRunner{
		errors: map[string]error{
			"tmux kill-session -t paracell-myapp-123": errors.New("exit status 1: can't find session: paracell-myapp-123"),
		},
	}
	adapter := TmuxAdapter{Runner: runner}
	cell := domain.Cell{Session: domain.Session{Name: "paracell-myapp-123"}}

	err := adapter.CleanSession(context.Background(), cell)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}
