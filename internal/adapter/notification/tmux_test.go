package notification

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/hgsg11/paracell/internal/domain"
)

func testResource(name string) domain.WorkspaceResource {
	return domain.NewWorkspaceResource(name, "130", "demo", "130", ".", nil)
}

func TestTmuxNotifierはメッセージ未設定なら何もしない(t *testing.T) {
	runner := &recordingRunner{}
	notifier := TmuxNotifier{Runner: runner}
	if err := notifier.NotifyReady(context.Background(), testResource("paracell-demo-123"), ""); err != nil {
		t.Fatalf("NotifyReadyでエラーが返った: %v", err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("calls = %#v, want none", runner.calls)
	}
}

func TestTmuxNotifierは対象sessionの各clientへstatusMessageを送る(t *testing.T) {
	runner := &recordingRunner{outputByArgs: map[string]outputResult{
		"tmux -L paracell-demo list-clients -t cell-130 -F #{client_tty}": {value: "/dev/pts/4\n/dev/pts/7\n"},
	}}
	notifier := TmuxNotifier{Runner: runner}
	if err := notifier.NotifyReady(context.Background(), testResource("cell-130"), "Ready: cell's name"); err != nil {
		t.Fatalf("NotifyReadyでエラーが返った: %v", err)
	}
	want := []runnerCall{
		{name: "tmux", args: []string{"-L", "paracell-demo", "display-message", "-c", "/dev/pts/4", "Ready: cell's name"}},
		{name: "tmux", args: []string{"-L", "paracell-demo", "display-message", "-c", "/dev/pts/7", "Ready: cell's name"}},
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestTmuxNotifierは対象sessionにclientがいないとき通知しない(t *testing.T) {
	runner := &recordingRunner{outputByArgs: map[string]outputResult{
		"tmux -L paracell-demo list-clients -t cell-130 -F #{client_tty}": {},
	}}
	notifier := TmuxNotifier{Runner: runner}
	if err := notifier.NotifyReady(context.Background(), testResource("cell-130"), "Ready: cell"); err != nil {
		t.Fatalf("NotifyReadyでエラーが返った: %v", err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("calls = %#v, want none", runner.calls)
	}
}

func TestTmuxNotifierは専用serverにsessionがない既存cellならdefaultServerへ通知する(t *testing.T) {
	runner := &recordingRunner{outputByArgs: map[string]outputResult{
		"tmux -L paracell-demo list-clients -t cell-130 -F #{client_tty}": {err: errors.New("can't find session: cell-130")},
		"tmux list-clients -t cell-130 -F #{client_tty}":                  {value: "/dev/pts/4\n"},
	}}
	if err := (TmuxNotifier{Runner: runner}).NotifyReady(context.Background(), testResource("cell-130"), "Ready: cell"); err != nil {
		t.Fatalf("NotifyReadyでエラーが返った: %v", err)
	}
	want := []runnerCall{{name: "tmux", args: []string{"display-message", "-c", "/dev/pts/4", "Ready: cell"}}}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestTmuxNotifierはどのserverにもsessionがなければエラーを返す(t *testing.T) {
	runner := &recordingRunner{outputByArgs: map[string]outputResult{
		"tmux -L paracell-demo list-clients -t cell-130 -F #{client_tty}": {err: errors.New("no server running")},
		"tmux list-clients -t cell-130 -F #{client_tty}":                  {err: errors.New("can't find session: cell-130")},
	}}
	if err := (TmuxNotifier{Runner: runner}).NotifyReady(context.Background(), testResource("cell-130"), "Ready: cell"); err == nil {
		t.Fatal("NotifyReadyでエラーが返らなかった")
	}
}

func TestTmuxNotifierはrootSessionのclientにも通知する(t *testing.T) {
	runner := &recordingRunner{outputByArgs: map[string]outputResult{
		"tmux -L paracell-demo list-clients -t cell-130 -F #{client_tty}": {value: "/dev/pts/4\n"},
		"tmux list-clients -t demo-root -F #{client_tty}":                 {value: "/dev/pts/9\n"},
	}}
	if err := (TmuxNotifier{Runner: runner}).NotifyReady(context.Background(), testResource("cell-130"), "Ready: cell"); err != nil {
		t.Fatalf("NotifyReadyでエラーが返った: %v", err)
	}
	want := []runnerCall{
		{name: "tmux", args: []string{"-L", "paracell-demo", "display-message", "-c", "/dev/pts/4", "Ready: cell"}},
		{name: "tmux", args: []string{"display-message", "-c", "/dev/pts/9", "Ready: cell"}},
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestTmuxNotifierはrootSessionがなくてもエラーにしない(t *testing.T) {
	runner := &recordingRunner{outputByArgs: map[string]outputResult{
		"tmux -L paracell-demo list-clients -t cell-130 -F #{client_tty}": {value: "/dev/pts/4\n"},
		"tmux list-clients -t demo-root -F #{client_tty}":                 {err: errors.New("no server running")},
	}}
	if err := (TmuxNotifier{Runner: runner}).NotifyReady(context.Background(), testResource("cell-130"), "Ready: cell"); err != nil {
		t.Fatalf("NotifyReadyでエラーが返った: %v", err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("calls = %#v, want only the cell client", runner.calls)
	}
}

type runnerCall struct {
	name string
	args []string
}

type recordingRunner struct {
	calls        []runnerCall
	output       string
	outputByArgs map[string]outputResult
}

func (r *recordingRunner) Run(ctx context.Context, name string, args ...string) error {
	_ = ctx
	r.calls = append(r.calls, runnerCall{name: name, args: append([]string(nil), args...)})
	return nil
}

func (r *recordingRunner) Output(ctx context.Context, name string, args ...string) (string, error) {
	_ = ctx
	key := strings.Join(append([]string{name}, args...), " ")
	if result, ok := r.outputByArgs[key]; ok {
		return result.value, result.err
	}
	return r.output, nil
}

type outputResult struct {
	value string
	err   error
}
