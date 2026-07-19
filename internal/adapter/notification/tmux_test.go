package notification

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/hgsg11/paracell/internal/domain"
)

func TestTmuxNotifierはdisplayMessageを実行する(t *testing.T) {
	t.Setenv("TMUX", "")
	runner := &recordingRunner{}
	notifier := TmuxNotifier{Runner: runner}
	runner.output = "/dev/ttys009"

	err := notifier.NotifyReady(context.Background(), domain.Cell{
		Name:    "123",
		Session: domain.Session{Name: "paracell-demo-123"},
	}, "ready 123")
	if err != nil {
		t.Fatalf("NotifyReadyでエラーが返った: %v", err)
	}

	want := []runnerCall{
		{name: "tmux", args: []string{"display-message", "-c", "/dev/ttys009", "ready 123"}},
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestTmuxNotifierはTmux内では現在のclientを使う(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-123/default,123,0")
	runner := &recordingRunner{}
	notifier := TmuxNotifier{Runner: runner}
	runner.output = "/dev/ttys010"

	err := notifier.NotifyReady(context.Background(), domain.Cell{
		Name:    "123",
		Session: domain.Session{Name: "paracell-demo-123"},
	}, "ready 123")
	if err != nil {
		t.Fatalf("NotifyReadyでエラーが返った: %v", err)
	}

	want := []runnerCall{
		{name: "tmux", args: []string{"display-message", "-c", "/dev/ttys010", "ready 123"}},
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestTmuxNotifierは現在clientが取れない場合sessionのclientへフォールバックする(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-123/default,123,0")
	runner := &recordingRunner{
		outputByArgs: map[string]outputResult{
			"tmux display-message -p #{client_tty}":                   {err: context.DeadlineExceeded},
			"tmux list-clients -t paracell-demo-123 -F #{client_tty}": {value: "/dev/ttys011\n/dev/ttys012"},
		},
	}
	notifier := TmuxNotifier{Runner: runner}

	err := notifier.NotifyReady(context.Background(), domain.Cell{
		Name:    "123",
		Session: domain.Session{Name: "paracell-demo-123"},
	}, "ready 123")
	if err != nil {
		t.Fatalf("NotifyReadyでエラーが返った: %v", err)
	}

	want := []runnerCall{
		{name: "tmux", args: []string{"display-message", "-c", "/dev/ttys011", "ready 123"}},
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestTmuxNotifierはメッセージ未設定なら何もしない(t *testing.T) {
	runner := &recordingRunner{}
	notifier := TmuxNotifier{Runner: runner}
	if err := notifier.NotifyReady(context.Background(), domain.Cell{Name: "123"}, ""); err != nil {
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
