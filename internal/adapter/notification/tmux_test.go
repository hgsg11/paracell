package notification

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

func TestTmuxNotifierはメッセージ未設定なら何もしない(t *testing.T) {
	runner := &recordingRunner{}
	notifier := TmuxNotifier{Runner: runner}
	if err := notifier.NotifyReady(context.Background(), "paracell-demo-123", ""); err != nil {
		t.Fatalf("NotifyReadyでエラーが返った: %v", err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("calls = %#v, want none", runner.calls)
	}
}

func TestTmuxNotifierは対象sessionの各clientへstatusMessageを送る(t *testing.T) {
	runner := &recordingRunner{outputByArgs: map[string]outputResult{
		"tmux list-clients -t cell-130 -F #{client_tty}": {value: "/dev/pts/4\n/dev/pts/7\n"},
	}}
	notifier := TmuxNotifier{Runner: runner}
	if err := notifier.NotifyReady(context.Background(), "cell-130", "Ready: cell's name"); err != nil {
		t.Fatalf("NotifyReadyでエラーが返った: %v", err)
	}
	want := []runnerCall{
		{name: "tmux", args: []string{"display-message", "-c", "/dev/pts/4", "Ready: cell's name"}},
		{name: "tmux", args: []string{"display-message", "-c", "/dev/pts/7", "Ready: cell's name"}},
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestTmuxNotifierは対象sessionにclientがいないとき通知しない(t *testing.T) {
	runner := &recordingRunner{outputByArgs: map[string]outputResult{
		"tmux list-clients -t cell-130 -F #{client_tty}": {},
	}}
	notifier := TmuxNotifier{Runner: runner}
	if err := notifier.NotifyReady(context.Background(), "cell-130", "Ready: cell"); err != nil {
		t.Fatalf("NotifyReadyでエラーが返った: %v", err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("calls = %#v, want none", runner.calls)
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
