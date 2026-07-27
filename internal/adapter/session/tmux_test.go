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

func appearanceCalls(target string, project string, label string, windowTargets ...string) []string {
	calls := []string{
		"tmux set-option -t " + target + " @paracell-project " + project,
		"tmux set-option -t " + target + " @paracell-status-label " + label,
		"tmux set-option -t " + target + " set-titles on",
		"tmux set-option -t " + target + " set-titles-string #{@paracell-project}",
		"tmux set-option -t " + target + " status-left #{@paracell-status-label} ",
		"tmux set-option -t " + target + " status-left-length 100",
		"tmux set-option -t " + target + " status-right #{?window_bigger,[#{window_offset_x}#,#{window_offset_y}] ,}%H:%M %d-%b-%y",
	}
	for _, windowTarget := range windowTargets {
		calls = append(calls,
			"tmux set-window-option -t "+windowTarget+" window-status-format #{@paracell-status-label}:#W#{?window_flags,#{window_flags}, }",
			"tmux set-window-option -t "+windowTarget+" window-status-current-format #{@paracell-status-label}:#W#{?window_flags,#{window_flags}, }",
		)
	}
	calls = append(calls, "tmux set-hook -t "+target+" after-new-window[100] set-window-option window-status-format '#{@paracell-status-label}:#W#{?window_flags,#{window_flags}, }'; set-window-option window-status-current-format '#{@paracell-status-label}:#W#{?window_flags,#{window_flags}, }'")
	return calls
}

func TestEnterSessionはTMUX外ならattachSessionを使う(t *testing.T) {
	t.Setenv("TMUX", "")
	runner := &fakeRunner{}
	adapter := TmuxAdapter{Runner: runner}
	cell := domain.Cell{Name: "123", Session: domain.Session{Name: "paracell-myapp-123"}}

	if err := adapter.EnterSession(context.Background(), cell); err != nil {
		t.Fatalf("EnterSessionでエラーが返った: %v", err)
	}
	want := appearanceCalls("paracell-myapp-123", "paracell-myapp", "123", "paracell-myapp-123")
	want = append(want,
		"tmux set-option -t paracell-myapp-123 key-table paracell",
		"tmux set-option -t paracell-myapp-123 mouse on",
		"tmux set-option -t paracell-myapp-123 set-clipboard on",
		"tmux bind-key -T paracell MouseDown1Pane select-pane -t = \\; send-keys -M",
		"tmux bind-key -T paracell MouseDrag1Pane if-shell -F #{||:#{pane_in_mode},#{mouse_any_flag}} send-keys -M copy-mode -M",
		"tmux bind-key -T paracell WheelUpPane if-shell -F #{||:#{alternate_on},#{pane_in_mode},#{mouse_any_flag}} send-keys -M copy-mode -e",
		"tmux bind-key -T paracell C-t next-window",
		"tmux bind-key -T paracell C-p display-popup -w 65 -h 50% -E paracell view",
		"tmux attach-session -E -t paracell-myapp-123",
	)
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestEnterSessionはTMUX内ならswitchClientを使う(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-1000/default,123,0")
	runner := &fakeRunner{}
	adapter := TmuxAdapter{Runner: runner}
	cell := domain.Cell{Name: "123", Session: domain.Session{Name: "paracell-myapp-123"}}

	if err := adapter.EnterSession(context.Background(), cell); err != nil {
		t.Fatalf("EnterSessionでエラーが返った: %v", err)
	}
	want := appearanceCalls("paracell-myapp-123", "paracell-myapp", "123", "paracell-myapp-123")
	want = append(want,
		"tmux set-option -t paracell-myapp-123 key-table paracell",
		"tmux set-option -t paracell-myapp-123 mouse on",
		"tmux set-option -t paracell-myapp-123 set-clipboard on",
		"tmux bind-key -T paracell MouseDown1Pane select-pane -t = \\; send-keys -M",
		"tmux bind-key -T paracell MouseDrag1Pane if-shell -F #{||:#{pane_in_mode},#{mouse_any_flag}} send-keys -M copy-mode -M",
		"tmux bind-key -T paracell WheelUpPane if-shell -F #{||:#{alternate_on},#{pane_in_mode},#{mouse_any_flag}} send-keys -M copy-mode -e",
		"tmux bind-key -T paracell C-t next-window",
		"tmux bind-key -T paracell C-p display-popup -w 65 -h 50% -E paracell view",
		"tmux switch-client -E -t paracell-myapp-123",
	)
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestExitSessionはTMUXClientをDetachする(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-1000/default,123,0")
	runner := &fakeRunner{}
	adapter := TmuxAdapter{Runner: runner}

	if err := adapter.ExitSession(context.Background()); err != nil {
		t.Fatalf("ExitSessionでエラーが返った: %v", err)
	}
	want := []string{"tmux detach-client"}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestExitSessionはTMUX外ではエラーにする(t *testing.T) {
	t.Setenv("TMUX", "")
	runner := &fakeRunner{}
	adapter := TmuxAdapter{Runner: runner}

	err := adapter.ExitSession(context.Background())
	if err == nil {
		t.Fatal("ExitSessionでエラーが返らなかった")
	}
	if err.Error() != "paracell exit must be run inside tmux" {
		t.Fatalf("error = %q, want %q", err.Error(), "paracell exit must be run inside tmux")
	}
	if len(runner.calls) != 0 {
		t.Fatalf("calls = %#v, want no calls", runner.calls)
	}
}

func TestEnterRootSessionはSessionがなければ作成してAttachする(t *testing.T) {
	t.Setenv("TMUX", "")
	runner := &fakeRunner{
		errors: map[string]error{
			"tmux has-session -t myapp-root": errors.New("exit status 1: can't find session: myapp-root"),
		},
	}
	adapter := TmuxAdapter{Runner: runner}

	if err := adapter.EnterRootSession(context.Background(), domain.ProjectConfig{Name: "myapp"}); err != nil {
		t.Fatalf("EnterRootSessionでエラーが返った: %v", err)
	}
	want := []string{
		"tmux has-session -t myapp-root",
		"tmux new-session -d -s myapp-root -c .",
		"tmux set-option -t myapp-root @paracell-project myapp",
		"tmux set-option -t myapp-root @paracell-status-label root",
		"tmux set-option -t myapp-root set-titles on",
		"tmux set-option -t myapp-root set-titles-string #{@paracell-project}",
		"tmux set-option -t myapp-root status-left #{@paracell-status-label} ",
		"tmux set-option -t myapp-root status-left-length 100",
		"tmux set-option -t myapp-root status-right #{?window_bigger,[#{window_offset_x}#,#{window_offset_y}] ,}%H:%M %d-%b-%y",
		"tmux set-window-option -t myapp-root window-status-format #{@paracell-status-label}:#W#{?window_flags,#{window_flags}, }",
		"tmux set-window-option -t myapp-root window-status-current-format #{@paracell-status-label}:#W#{?window_flags,#{window_flags}, }",
		"tmux set-hook -t myapp-root after-new-window[100] set-window-option window-status-format '#{@paracell-status-label}:#W#{?window_flags,#{window_flags}, }'; set-window-option window-status-current-format '#{@paracell-status-label}:#W#{?window_flags,#{window_flags}, }'",
		"tmux set-option -t myapp-root key-table paracell",
		"tmux set-option -t myapp-root mouse on",
		"tmux set-option -t myapp-root set-clipboard on",
		"tmux bind-key -T paracell MouseDown1Pane select-pane -t = \\; send-keys -M",
		"tmux bind-key -T paracell MouseDrag1Pane if-shell -F #{||:#{pane_in_mode},#{mouse_any_flag}} send-keys -M copy-mode -M",
		"tmux bind-key -T paracell WheelUpPane if-shell -F #{||:#{alternate_on},#{pane_in_mode},#{mouse_any_flag}} send-keys -M copy-mode -e",
		"tmux bind-key -T paracell C-t next-window",
		"tmux bind-key -T paracell C-p display-popup -w 65 -h 50% -E paracell view",
		"tmux attach-session -E -t myapp-root",
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestEnterRootSessionはPopup起動用にProjectRootを引き回す(t *testing.T) {
	t.Setenv("TMUX", "")
	runner := &fakeRunner{
		errors: map[string]error{
			"tmux has-session -t myapp-root": errors.New("exit status 1: can't find session: myapp-root"),
		},
	}
	adapter := TmuxAdapter{Runner: runner, Root: "/project"}

	if err := adapter.EnterRootSession(context.Background(), domain.ProjectConfig{Name: "myapp"}); err != nil {
		t.Fatalf("EnterRootSessionでエラーが返った: %v", err)
	}
	want := []string{
		"tmux has-session -t myapp-root",
		"tmux new-session -d -s myapp-root -e PARACELL_ROOT=/project -c /project",
		"tmux set-option -t myapp-root @paracell-project myapp",
		"tmux set-option -t myapp-root @paracell-status-label root",
		"tmux set-option -t myapp-root set-titles on",
		"tmux set-option -t myapp-root set-titles-string #{@paracell-project}",
		"tmux set-option -t myapp-root status-left #{@paracell-status-label} ",
		"tmux set-option -t myapp-root status-left-length 100",
		"tmux set-option -t myapp-root status-right #{?window_bigger,[#{window_offset_x}#,#{window_offset_y}] ,}%H:%M %d-%b-%y",
		"tmux set-window-option -t myapp-root window-status-format #{@paracell-status-label}:#W#{?window_flags,#{window_flags}, }",
		"tmux set-window-option -t myapp-root window-status-current-format #{@paracell-status-label}:#W#{?window_flags,#{window_flags}, }",
		"tmux set-hook -t myapp-root after-new-window[100] set-window-option window-status-format '#{@paracell-status-label}:#W#{?window_flags,#{window_flags}, }'; set-window-option window-status-current-format '#{@paracell-status-label}:#W#{?window_flags,#{window_flags}, }'",
		"tmux set-option -t myapp-root key-table paracell",
		"tmux set-option -t myapp-root mouse on",
		"tmux set-option -t myapp-root set-clipboard on",
		"tmux bind-key -T paracell MouseDown1Pane select-pane -t = \\; send-keys -M",
		"tmux bind-key -T paracell MouseDrag1Pane if-shell -F #{||:#{pane_in_mode},#{mouse_any_flag}} send-keys -M copy-mode -M",
		"tmux bind-key -T paracell WheelUpPane if-shell -F #{||:#{alternate_on},#{pane_in_mode},#{mouse_any_flag}} send-keys -M copy-mode -e",
		"tmux bind-key -T paracell C-t next-window",
		"tmux bind-key -T paracell C-p display-popup -w 65 -h 50% -d /project -E env PARACELL_ROOT=/project paracell view",
		"tmux attach-session -E -t myapp-root",
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestEnterRootSessionはHasSessionがexitStatus1だけでも作成してAttachする(t *testing.T) {
	t.Setenv("TMUX", "")
	runner := &fakeRunner{
		errors: map[string]error{
			"tmux has-session -t myapp-root": &exec.ExitError{},
		},
	}
	adapter := TmuxAdapter{Runner: runner}

	if err := adapter.EnterRootSession(context.Background(), domain.ProjectConfig{Name: "myapp"}); err != nil {
		t.Fatalf("EnterRootSessionでエラーが返った: %v", err)
	}
	want := []string{
		"tmux has-session -t myapp-root",
		"tmux new-session -d -s myapp-root -c .",
		"tmux set-option -t myapp-root @paracell-project myapp",
		"tmux set-option -t myapp-root @paracell-status-label root",
		"tmux set-option -t myapp-root set-titles on",
		"tmux set-option -t myapp-root set-titles-string #{@paracell-project}",
		"tmux set-option -t myapp-root status-left #{@paracell-status-label} ",
		"tmux set-option -t myapp-root status-left-length 100",
		"tmux set-option -t myapp-root status-right #{?window_bigger,[#{window_offset_x}#,#{window_offset_y}] ,}%H:%M %d-%b-%y",
		"tmux set-window-option -t myapp-root window-status-format #{@paracell-status-label}:#W#{?window_flags,#{window_flags}, }",
		"tmux set-window-option -t myapp-root window-status-current-format #{@paracell-status-label}:#W#{?window_flags,#{window_flags}, }",
		"tmux set-hook -t myapp-root after-new-window[100] set-window-option window-status-format '#{@paracell-status-label}:#W#{?window_flags,#{window_flags}, }'; set-window-option window-status-current-format '#{@paracell-status-label}:#W#{?window_flags,#{window_flags}, }'",
		"tmux set-option -t myapp-root key-table paracell",
		"tmux set-option -t myapp-root mouse on",
		"tmux set-option -t myapp-root set-clipboard on",
		"tmux bind-key -T paracell MouseDown1Pane select-pane -t = \\; send-keys -M",
		"tmux bind-key -T paracell MouseDrag1Pane if-shell -F #{||:#{pane_in_mode},#{mouse_any_flag}} send-keys -M copy-mode -M",
		"tmux bind-key -T paracell WheelUpPane if-shell -F #{||:#{alternate_on},#{pane_in_mode},#{mouse_any_flag}} send-keys -M copy-mode -e",
		"tmux bind-key -T paracell C-t next-window",
		"tmux bind-key -T paracell C-p display-popup -w 65 -h 50% -E paracell view",
		"tmux attach-session -E -t myapp-root",
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
		"tmux has-session -t myapp-root",
		"tmux set-option -t myapp-root @paracell-project myapp",
		"tmux set-option -t myapp-root @paracell-status-label root",
		"tmux set-option -t myapp-root set-titles on",
		"tmux set-option -t myapp-root set-titles-string #{@paracell-project}",
		"tmux set-option -t myapp-root status-left #{@paracell-status-label} ",
		"tmux set-option -t myapp-root status-left-length 100",
		"tmux set-option -t myapp-root status-right #{?window_bigger,[#{window_offset_x}#,#{window_offset_y}] ,}%H:%M %d-%b-%y",
		"tmux set-window-option -t myapp-root window-status-format #{@paracell-status-label}:#W#{?window_flags,#{window_flags}, }",
		"tmux set-window-option -t myapp-root window-status-current-format #{@paracell-status-label}:#W#{?window_flags,#{window_flags}, }",
		"tmux set-hook -t myapp-root after-new-window[100] set-window-option window-status-format '#{@paracell-status-label}:#W#{?window_flags,#{window_flags}, }'; set-window-option window-status-current-format '#{@paracell-status-label}:#W#{?window_flags,#{window_flags}, }'",
		"tmux set-option -t myapp-root key-table paracell",
		"tmux set-option -t myapp-root mouse on",
		"tmux set-option -t myapp-root set-clipboard on",
		"tmux bind-key -T paracell MouseDown1Pane select-pane -t = \\; send-keys -M",
		"tmux bind-key -T paracell MouseDrag1Pane if-shell -F #{||:#{pane_in_mode},#{mouse_any_flag}} send-keys -M copy-mode -M",
		"tmux bind-key -T paracell WheelUpPane if-shell -F #{||:#{alternate_on},#{pane_in_mode},#{mouse_any_flag}} send-keys -M copy-mode -e",
		"tmux bind-key -T paracell C-t next-window",
		"tmux bind-key -T paracell C-p display-popup -w 65 -h 50% -d /project -E env PARACELL_ROOT=/project paracell view",
		"tmux attach-session -E -t myapp-root",
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestEnterRootSessionはTMUX内なら環境を更新せずswitchClientを使う(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-1000/default,123,0")
	runner := &fakeRunner{}
	adapter := TmuxAdapter{Runner: runner}

	if err := adapter.EnterRootSession(context.Background(), domain.ProjectConfig{Name: "myapp"}); err != nil {
		t.Fatalf("EnterRootSessionでエラーが返った: %v", err)
	}
	if got := runner.calls[len(runner.calls)-1]; got != "tmux switch-client -E -t myapp-root" {
		t.Fatalf("last call = %q, want %q", got, "tmux switch-client -E -t myapp-root")
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
		"tmux set-option -t paracell-myapp-123 @paracell-project paracell-myapp",
		"tmux set-option -t paracell-myapp-123 @paracell-status-label 123",
		"tmux set-option -t paracell-myapp-123 set-titles on",
		"tmux set-option -t paracell-myapp-123 set-titles-string #{@paracell-project}",
		"tmux set-option -t paracell-myapp-123 status-left #{@paracell-status-label} ",
		"tmux set-option -t paracell-myapp-123 status-left-length 100",
		"tmux set-option -t paracell-myapp-123 status-right #{?window_bigger,[#{window_offset_x}#,#{window_offset_y}] ,}%H:%M %d-%b-%y",
		"tmux set-window-option -t paracell-myapp-123 window-status-format #{@paracell-status-label}:#W#{?window_flags,#{window_flags}, }",
		"tmux set-window-option -t paracell-myapp-123 window-status-current-format #{@paracell-status-label}:#W#{?window_flags,#{window_flags}, }",
		"tmux set-hook -t paracell-myapp-123 after-new-window[100] set-window-option window-status-format '#{@paracell-status-label}:#W#{?window_flags,#{window_flags}, }'; set-window-option window-status-current-format '#{@paracell-status-label}:#W#{?window_flags,#{window_flags}, }'",
		"tmux set-option -t paracell-myapp-123 key-table paracell",
		"tmux set-option -t paracell-myapp-123 mouse on",
		"tmux set-option -t paracell-myapp-123 set-clipboard on",
		"tmux bind-key -T paracell MouseDown1Pane select-pane -t = \\; send-keys -M",
		"tmux bind-key -T paracell MouseDrag1Pane if-shell -F #{||:#{pane_in_mode},#{mouse_any_flag}} send-keys -M copy-mode -M",
		"tmux bind-key -T paracell WheelUpPane if-shell -F #{||:#{alternate_on},#{pane_in_mode},#{mouse_any_flag}} send-keys -M copy-mode -e",
		"tmux bind-key -T paracell C-t next-window",
		"tmux bind-key -T paracell C-p display-popup -w 65 -h 50% -d /project -E env PARACELL_ROOT=/project paracell view",
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
		"tmux set-option -t paracell-myapp-123 @paracell-project paracell-myapp",
		"tmux set-option -t paracell-myapp-123 @paracell-status-label 123",
		"tmux set-option -t paracell-myapp-123 set-titles on",
		"tmux set-option -t paracell-myapp-123 set-titles-string #{@paracell-project}",
		"tmux set-option -t paracell-myapp-123 status-left #{@paracell-status-label} ",
		"tmux set-option -t paracell-myapp-123 status-left-length 100",
		"tmux set-option -t paracell-myapp-123 status-right #{?window_bigger,[#{window_offset_x}#,#{window_offset_y}] ,}%H:%M %d-%b-%y",
		"tmux set-window-option -t paracell-myapp-123:editor window-status-format #{@paracell-status-label}:#W#{?window_flags,#{window_flags}, }",
		"tmux set-window-option -t paracell-myapp-123:editor window-status-current-format #{@paracell-status-label}:#W#{?window_flags,#{window_flags}, }",
		"tmux set-window-option -t paracell-myapp-123:server window-status-format #{@paracell-status-label}:#W#{?window_flags,#{window_flags}, }",
		"tmux set-window-option -t paracell-myapp-123:server window-status-current-format #{@paracell-status-label}:#W#{?window_flags,#{window_flags}, }",
		"tmux set-hook -t paracell-myapp-123 after-new-window[100] set-window-option window-status-format '#{@paracell-status-label}:#W#{?window_flags,#{window_flags}, }'; set-window-option window-status-current-format '#{@paracell-status-label}:#W#{?window_flags,#{window_flags}, }'",
		"tmux set-option -t paracell-myapp-123 key-table paracell",
		"tmux set-option -t paracell-myapp-123 mouse on",
		"tmux set-option -t paracell-myapp-123 set-clipboard on",
		"tmux bind-key -T paracell MouseDown1Pane select-pane -t = \\; send-keys -M",
		"tmux bind-key -T paracell MouseDrag1Pane if-shell -F #{||:#{pane_in_mode},#{mouse_any_flag}} send-keys -M copy-mode -M",
		"tmux bind-key -T paracell WheelUpPane if-shell -F #{||:#{alternate_on},#{pane_in_mode},#{mouse_any_flag}} send-keys -M copy-mode -e",
		"tmux bind-key -T paracell C-t next-window",
		"tmux bind-key -T paracell C-p display-popup -w 65 -h 50% -d /project -E env PARACELL_ROOT=/project paracell view",
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
		"tmux set-option -t paracell-myapp-123 @paracell-project paracell-myapp",
		"tmux set-option -t paracell-myapp-123 @paracell-status-label 123",
		"tmux set-option -t paracell-myapp-123 set-titles on",
		"tmux set-option -t paracell-myapp-123 set-titles-string #{@paracell-project}",
		"tmux set-option -t paracell-myapp-123 status-left #{@paracell-status-label} ",
		"tmux set-option -t paracell-myapp-123 status-left-length 100",
		"tmux set-option -t paracell-myapp-123 status-right #{?window_bigger,[#{window_offset_x}#,#{window_offset_y}] ,}%H:%M %d-%b-%y",
		"tmux set-window-option -t paracell-myapp-123:editor window-status-format #{@paracell-status-label}:#W#{?window_flags,#{window_flags}, }",
		"tmux set-window-option -t paracell-myapp-123:editor window-status-current-format #{@paracell-status-label}:#W#{?window_flags,#{window_flags}, }",
		"tmux set-window-option -t paracell-myapp-123:server window-status-format #{@paracell-status-label}:#W#{?window_flags,#{window_flags}, }",
		"tmux set-window-option -t paracell-myapp-123:server window-status-current-format #{@paracell-status-label}:#W#{?window_flags,#{window_flags}, }",
		"tmux set-window-option -t paracell-myapp-123:test window-status-format #{@paracell-status-label}:#W#{?window_flags,#{window_flags}, }",
		"tmux set-window-option -t paracell-myapp-123:test window-status-current-format #{@paracell-status-label}:#W#{?window_flags,#{window_flags}, }",
		"tmux set-hook -t paracell-myapp-123 after-new-window[100] set-window-option window-status-format '#{@paracell-status-label}:#W#{?window_flags,#{window_flags}, }'; set-window-option window-status-current-format '#{@paracell-status-label}:#W#{?window_flags,#{window_flags}, }'",
		"tmux set-option -t paracell-myapp-123 key-table paracell",
		"tmux set-option -t paracell-myapp-123 mouse on",
		"tmux set-option -t paracell-myapp-123 set-clipboard on",
		"tmux bind-key -T paracell MouseDown1Pane select-pane -t = \\; send-keys -M",
		"tmux bind-key -T paracell MouseDrag1Pane if-shell -F #{||:#{pane_in_mode},#{mouse_any_flag}} send-keys -M copy-mode -M",
		"tmux bind-key -T paracell WheelUpPane if-shell -F #{||:#{alternate_on},#{pane_in_mode},#{mouse_any_flag}} send-keys -M copy-mode -e",
		"tmux bind-key -T paracell C-t next-window",
		"tmux bind-key -T paracell C-p display-popup -w 65 -h 50% -d /project -E env PARACELL_ROOT=/project paracell view",
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
