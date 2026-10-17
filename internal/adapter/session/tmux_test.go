package session

import (
	"context"
	"errors"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/hgsg11/paracell/internal/domain"
)

type fakeRunner struct {
	calls            []string
	environmentCalls []string
	errors           map[string]error
	outputs          map[string]string
	outputErrors     map[string]error
}

func (r *fakeRunner) Run(ctx context.Context, name string, args ...string) error {
	_ = ctx
	call := name + " " + joinArgs(args)
	subcommand := args
	if len(subcommand) > 1 && subcommand[0] == "-L" {
		subcommand = subcommand[2:]
	}
	if name == "tmux" && len(subcommand) > 0 && subcommand[0] == "set-environment" {
		r.environmentCalls = append(r.environmentCalls, call)
		if r.errors != nil && r.errors[call] != nil {
			return r.errors[call]
		}
		return nil
	}
	r.calls = append(r.calls, call)
	if r.errors != nil && r.errors[call] != nil {
		return r.errors[call]
	}
	return nil
}

func (r *fakeRunner) Output(ctx context.Context, name string, args ...string) (string, error) {
	_ = ctx
	call := name + " " + joinArgs(args)
	return r.outputs[call], r.outputErrors[call]
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
		"tmux -L paracell-paracell-myapp set-option -t " + target + " @paracell-project " + project,
		"tmux -L paracell-paracell-myapp set-option -t " + target + " @paracell-status-label " + label,
		"tmux -L paracell-paracell-myapp set-option -t " + target + " set-titles on",
		"tmux -L paracell-paracell-myapp set-option -t " + target + " set-titles-string #{@paracell-project}",
		"tmux -L paracell-paracell-myapp set-option -t " + target + " status-left #{@paracell-status-label} ",
		"tmux -L paracell-paracell-myapp set-option -t " + target + " status-left-length 100",
		"tmux -L paracell-paracell-myapp set-option -t " + target + " status-right #{?window_bigger,[#{window_offset_x}#,#{window_offset_y}] ,}%H:%M %d-%b-%y",
	}
	for _, windowTarget := range windowTargets {
		calls = append(calls,
			"tmux -L paracell-paracell-myapp set-window-option -t "+windowTarget+" window-status-format #W#{?window_flags,#{window_flags}, }",
			"tmux -L paracell-paracell-myapp set-window-option -t "+windowTarget+" window-status-current-format #W#{?window_flags,#{window_flags}, }",
		)
	}
	calls = append(calls, "tmux -L paracell-paracell-myapp set-hook -t "+target+" after-new-window[100] set-window-option window-status-format '#W#{?window_flags,#{window_flags}, }'; set-window-option window-status-current-format '#W#{?window_flags,#{window_flags}, }'")
	return calls
}

func cellWorkspaceResource(note string, windows []domain.WorkspaceWindow) domain.WorkspaceResource {
	label := "123"
	if note != "" {
		label = note
	}
	return domain.NewWorkspaceResource("paracell-myapp-123", "123", "paracell-myapp", label, ".paracell/cells/123/source", windows)
}

func namedWorkspaceResource(name string, windows []domain.WorkspaceWindow) domain.WorkspaceResource {
	return domain.NewWorkspaceResource("paracell-myapp-"+name, name, "paracell-myapp", name, ".paracell/cells/"+name+"/source", windows)
}

func TestEnterWorkspaceはTMUX外ならattachWorkspaceを使う(t *testing.T) {
	t.Setenv("TMUX", "")
	runner := &fakeRunner{}
	adapter := TmuxAdapter{Runner: runner}
	cell := cellWorkspaceResource("", nil)

	if err := adapter.EnterWorkspace(context.Background(), cell); err != nil {
		t.Fatalf("EnterWorkspaceでエラーが返った: %v", err)
	}
	want := []string{"tmux -L paracell-paracell-myapp has-session -t =paracell-myapp-123"}
	want = append(want, appearanceCalls("paracell-myapp-123", "paracell-myapp", "123", "paracell-myapp-123")...)
	want = append(want,
		"tmux -L paracell-paracell-myapp set-option -t paracell-myapp-123 key-table paracell-paracell-myapp-123",
		"tmux -L paracell-paracell-myapp set-option -t paracell-myapp-123 mouse on",
		"tmux -L paracell-paracell-myapp set-option -t paracell-myapp-123 set-clipboard on",
		"tmux -L paracell-paracell-myapp bind-key -T paracell-paracell-myapp-123 MouseDown1Pane select-pane -t = \\; send-keys -M",
		"tmux -L paracell-paracell-myapp bind-key -T paracell-paracell-myapp-123 MouseDrag1Pane if-shell -F #{||:#{pane_in_mode},#{mouse_any_flag}} send-keys -M copy-mode -M",
		"tmux -L paracell-paracell-myapp bind-key -T paracell-paracell-myapp-123 WheelUpPane if-shell -F #{||:#{alternate_on},#{pane_in_mode},#{mouse_any_flag}} send-keys -M copy-mode -e",
		"tmux -L paracell-paracell-myapp bind-key -T paracell-paracell-myapp-123 C-t next-window",
		"tmux -L paracell-paracell-myapp bind-key -T paracell-paracell-myapp-123 C-p display-popup -t paracell-myapp-123 -w 65 -h 24 -E paracell view",
		"tmux -L paracell-paracell-myapp attach-session -E -t paracell-myapp-123",
	)
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestUpdateStatusLabelはNoteを優先しWorkspaceなしを識別する(t *testing.T) {
	cell := cellWorkspaceResource("API実装中", nil)
	runner := &fakeRunner{}
	adapter := TmuxAdapter{Runner: runner}
	if err := adapter.UpdateStatusLabel(context.Background(), cell); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"tmux -L paracell-paracell-myapp has-session -t =paracell-myapp-123",
		"tmux -L paracell-paracell-myapp set-option -t paracell-myapp-123 @paracell-status-label API実装中",
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}

	missingCall := "tmux -L paracell-paracell-myapp set-option -t paracell-myapp-123 @paracell-status-label API実装中"
	runner = &fakeRunner{errors: map[string]error{missingCall: errors.New("can't find session: paracell-myapp-123")}}
	err := (TmuxAdapter{Runner: runner}).UpdateStatusLabel(context.Background(), cell)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestUpdateStatusLabelは既存window名から繰り返されたlabelを除く(t *testing.T) {
	const target = "paracell-myapp-123"
	runner := &fakeRunner{outputs: map[string]string{
		"tmux -L paracell-paracell-myapp list-windows -t " + target + " -F #{window_id}\t#{window_name}": "%9\tnote:note:note:editor\n",
	}}
	if err := (TmuxAdapter{Runner: runner}).UpdateStatusLabel(context.Background(), cellWorkspaceResource("note", nil)); err != nil {
		t.Fatalf("UpdateStatusLabelでエラーが返った: %v", err)
	}
	if !containsCall(runner.calls, "tmux -L paracell-paracell-myapp rename-window -t %9 editor") {
		t.Fatalf("繰り返されたprefixを除くrenameがない: calls = %#v", runner.calls)
	}
}

func TestPrepareWorkspaceはNoteをStatusLabelへ反映する(t *testing.T) {
	for _, note := range []string{"API実装中", "root", ""} {
		t.Run(note, func(t *testing.T) {
			runner := &fakeRunner{}
			cell := cellWorkspaceResource(note, nil)
			if err := NewTmuxAdapter(runner, "").PrepareWorkspace(context.Background(), cell); err != nil {
				t.Fatal(err)
			}
			label := note
			if label == "" {
				label = "123"
			}
			for _, want := range appearanceCalls("paracell-myapp-123", "paracell-myapp", label, "paracell-myapp-123") {
				if !containsCall(runner.calls, want) {
					t.Fatalf("appearance call not found: calls = %#v, want %q", runner.calls, want)
				}
			}
		})
	}
}

func TestEnterWorkspaceはResurrect後のWorkspace環境を再設定する(t *testing.T) {
	t.Setenv("TMUX", "")
	runner := &fakeRunner{}
	adapter := TmuxAdapter{Runner: runner, Root: "/project"}
	cell := cellWorkspaceResource("", nil)

	if err := adapter.EnterWorkspace(context.Background(), cell); err != nil {
		t.Fatalf("EnterWorkspaceでエラーが返った: %v", err)
	}
	want := []string{
		"tmux -L paracell-paracell-myapp set-environment -t paracell-myapp-123 PARACELL_CELL 123",
		"tmux -L paracell-paracell-myapp set-environment -t paracell-myapp-123 PARACELL_ROOT /project",
	}
	if !reflect.DeepEqual(runner.environmentCalls, want) {
		t.Fatalf("environment calls = %#v, want %#v", runner.environmentCalls, want)
	}
}

func TestConfigureWorkspaceはContinuumのStatusRightを保つ(t *testing.T) {
	const target = "paracell-myapp-123"
	const continuum = "#(/plugins/tmux-continuum/scripts/continuum_save.sh)#[fg=green] online"
	runner := &fakeRunner{outputs: map[string]string{
		"tmux -L paracell-paracell-myapp show-option -v -t " + target + " status-right": continuum + "\n",
	}}
	adapter := TmuxAdapter{Runner: runner}

	if err := adapter.configureWorkspace(context.Background(), "paracell-paracell-myapp", target, "paracell-myapp", "123", []string{target}); err != nil {
		t.Fatalf("configureWorkspaceでエラーが返った: %v", err)
	}
	want := "tmux -L paracell-paracell-myapp set-option -t " + target + " status-right " + continuum + " " + paracellClockFormat
	if !containsCall(runner.calls, want) {
		t.Fatalf("status-right call not found: calls = %#v, want %q", runner.calls, want)
	}
}

func TestConfigureWorkspaceは時刻表示を重複追加しない(t *testing.T) {
	const target = "paracell-myapp-123"
	existing := "#(/plugins/tmux-continuum/scripts/continuum_save.sh) " + paracellClockFormat
	runner := &fakeRunner{outputs: map[string]string{
		"tmux -L paracell-paracell-myapp show-option -v -t " + target + " status-right": existing + "\n",
	}}
	adapter := TmuxAdapter{Runner: runner}

	if err := adapter.configureWorkspace(context.Background(), "paracell-paracell-myapp", target, "paracell-myapp", "123", []string{target}); err != nil {
		t.Fatalf("configureWorkspaceでエラーが返った: %v", err)
	}
	for _, call := range runner.calls {
		if strings.Contains(call, " status-right ") {
			t.Fatalf("status-rightを再設定した: %q", call)
		}
	}
}

func TestConfigureWorkspaceはwindow名から重複したstatusLabelを除く(t *testing.T) {
	const target = "paracell-myapp-123"
	runner := &fakeRunner{outputs: map[string]string{
		"tmux -L paracell-paracell-myapp show-option -v -t " + target + " status-right":                  paracellClockFormat + "\n",
		"tmux -L paracell-paracell-myapp list-windows -t " + target + " -F #{window_id}\t#{window_name}": "%9\tnote:editor\n%10\tserver\n",
	}}
	if err := (TmuxAdapter{Runner: runner}).configureWorkspace(context.Background(), "paracell-paracell-myapp", target, "paracell-myapp", "note", nil); err != nil {
		t.Fatalf("configureWorkspaceでエラーが返った: %v", err)
	}
	for _, want := range []string{
		"tmux -L paracell-paracell-myapp rename-window -t %9 editor",
		"tmux -L paracell-paracell-myapp set-window-option -t %9 window-status-format #W#{?window_flags,#{window_flags}, }",
		"tmux -L paracell-paracell-myapp set-window-option -t %10 window-status-format #W#{?window_flags,#{window_flags}, }",
	} {
		if !containsCall(runner.calls, want) {
			t.Fatalf("window label call not found: calls = %#v, want %q", runner.calls, want)
		}
	}
}

func TestCreateWorkspaceは途中失敗時に部分Workspaceを削除して再試行可能にする(t *testing.T) {
	createErr := errors.New("new window failed")
	runner := &fakeRunner{errors: map[string]error{
		"tmux -L paracell-paracell-myapp new-window -t paracell-myapp-123 -n server -c .paracell/cells/123/source": createErr,
	}}
	adapter := TmuxAdapter{Runner: runner, Root: "/project"}
	cell := cellWorkspaceResource("", []domain.WorkspaceWindow{
		{Name: "editor"}, {Name: "server"},
	})

	err := adapter.CreateWorkspace(context.Background(), cell)
	if !errors.Is(err, createErr) {
		t.Fatalf("error = %v", err)
	}
	if !containsCall(runner.calls, "tmux -L paracell-paracell-myapp kill-session -t paracell-myapp-123") {
		t.Fatalf("partial session was not removed: %#v", runner.calls)
	}
}

func TestEnterWorkspaceはResurrectで復元された全Windowを再設定する(t *testing.T) {
	t.Setenv("TMUX", "")
	const target = "paracell-myapp-123"
	runner := &fakeRunner{outputs: map[string]string{
		"tmux -L paracell-paracell-myapp list-windows -t " + target + " -F #{window_id}\t#{window_name}": "%9\n%10\n",
	}}
	adapter := TmuxAdapter{Runner: runner, Root: "/project"}
	cell := cellWorkspaceResource("", nil)

	if err := adapter.EnterWorkspace(context.Background(), cell); err != nil {
		t.Fatalf("EnterWorkspaceでエラーが返った: %v", err)
	}
	for _, windowID := range []string{"%9", "%10"} {
		want := "tmux -L paracell-paracell-myapp set-window-option -t " + windowID + " window-status-format #W#{?window_flags,#{window_flags}, }"
		if !containsCall(runner.calls, want) {
			t.Fatalf("復元windowのlabel再設定がない: calls = %#v, want %q", runner.calls, want)
		}
	}
	if !containsCall(runner.calls, "tmux -L paracell-paracell-myapp set-hook -t "+target+" after-new-window[100] set-window-option window-status-format '#W#{?window_flags,#{window_flags}, }'; set-window-option window-status-current-format '#W#{?window_flags,#{window_flags}, }'") {
		t.Fatalf("new-window hookが再設定されていない: calls = %#v", runner.calls)
	}
	if !containsCall(runner.calls, "tmux -L paracell-paracell-myapp set-option -t "+target+" key-table paracell-"+target) {
		t.Fatalf("key tableが再設定されていない: calls = %#v", runner.calls)
	}
}

func containsCall(calls []string, want string) bool {
	for _, call := range calls {
		if call == want {
			return true
		}
	}
	return false
}

func TestEnterWorkspaceはTMUX内ならswitchClientを使う(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-1000/default,123,0")
	runner := &fakeRunner{}
	adapter := TmuxAdapter{Runner: runner}
	cell := cellWorkspaceResource("", nil)

	if err := adapter.EnterWorkspace(context.Background(), cell); err != nil {
		t.Fatalf("EnterWorkspaceでエラーが返った: %v", err)
	}
	want := []string{"tmux -L paracell-paracell-myapp has-session -t =paracell-myapp-123"}
	want = append(want, appearanceCalls("paracell-myapp-123", "paracell-myapp", "123", "paracell-myapp-123")...)
	want = append(want,
		"tmux -L paracell-paracell-myapp set-option -t paracell-myapp-123 key-table paracell-paracell-myapp-123",
		"tmux -L paracell-paracell-myapp set-option -t paracell-myapp-123 mouse on",
		"tmux -L paracell-paracell-myapp set-option -t paracell-myapp-123 set-clipboard on",
		"tmux -L paracell-paracell-myapp bind-key -T paracell-paracell-myapp-123 MouseDown1Pane select-pane -t = \\; send-keys -M",
		"tmux -L paracell-paracell-myapp bind-key -T paracell-paracell-myapp-123 MouseDrag1Pane if-shell -F #{||:#{pane_in_mode},#{mouse_any_flag}} send-keys -M copy-mode -M",
		"tmux -L paracell-paracell-myapp bind-key -T paracell-paracell-myapp-123 WheelUpPane if-shell -F #{||:#{alternate_on},#{pane_in_mode},#{mouse_any_flag}} send-keys -M copy-mode -e",
		"tmux -L paracell-paracell-myapp bind-key -T paracell-paracell-myapp-123 C-t next-window",
		"tmux -L paracell-paracell-myapp bind-key -T paracell-paracell-myapp-123 C-p display-popup -t paracell-myapp-123 -w 65 -h 24 -E paracell view",
		"tmux detach-client -E 'tmux' '-L' 'paracell-paracell-myapp' 'attach-session' '-E' '-t' 'paracell-myapp-123'",
	)
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestExitWorkspaceはTMUXClientをDetachする(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-1000/default,123,0")
	runner := &fakeRunner{}
	adapter := TmuxAdapter{Runner: runner}

	if err := adapter.ExitWorkspace(context.Background()); err != nil {
		t.Fatalf("ExitWorkspaceでエラーが返った: %v", err)
	}
	want := []string{"tmux detach-client"}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestExitWorkspaceはTMUX外ではエラーにする(t *testing.T) {
	t.Setenv("TMUX", "")
	runner := &fakeRunner{}
	adapter := TmuxAdapter{Runner: runner}

	err := adapter.ExitWorkspace(context.Background())
	if err == nil {
		t.Fatal("ExitWorkspaceでエラーが返らなかった")
	}
	if err.Error() != "paracell exit must be run inside tmux" {
		t.Fatalf("error = %q, want %q", err.Error(), "paracell exit must be run inside tmux")
	}
	if len(runner.calls) != 0 {
		t.Fatalf("calls = %#v, want no calls", runner.calls)
	}
}

func TestEnterRootWorkspaceはWorkspaceがなければ作成してAttachする(t *testing.T) {
	t.Setenv("TMUX", "")
	runner := &fakeRunner{
		errors: map[string]error{
			"tmux has-session -t myapp-root": errors.New("exit status 1: can't find session: myapp-root"),
		},
	}
	adapter := TmuxAdapter{Runner: runner}

	if err := adapter.EnterRootWorkspace(context.Background(), "myapp"); err != nil {
		t.Fatalf("EnterRootWorkspaceでエラーが返った: %v", err)
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
		"tmux set-window-option -t myapp-root window-status-format #W#{?window_flags,#{window_flags}, }",
		"tmux set-window-option -t myapp-root window-status-current-format #W#{?window_flags,#{window_flags}, }",
		"tmux set-hook -t myapp-root after-new-window[100] set-window-option window-status-format '#W#{?window_flags,#{window_flags}, }'; set-window-option window-status-current-format '#W#{?window_flags,#{window_flags}, }'",
		"tmux set-option -t myapp-root key-table paracell-myapp-root",
		"tmux set-option -t myapp-root mouse on",
		"tmux set-option -t myapp-root set-clipboard on",
		"tmux bind-key -T paracell-myapp-root MouseDown1Pane select-pane -t = \\; send-keys -M",
		"tmux bind-key -T paracell-myapp-root MouseDrag1Pane if-shell -F #{||:#{pane_in_mode},#{mouse_any_flag}} send-keys -M copy-mode -M",
		"tmux bind-key -T paracell-myapp-root WheelUpPane if-shell -F #{||:#{alternate_on},#{pane_in_mode},#{mouse_any_flag}} send-keys -M copy-mode -e",
		"tmux bind-key -T paracell-myapp-root C-t next-window",
		"tmux bind-key -T paracell-myapp-root C-p display-popup -t myapp-root -w 65 -h 24 -E paracell view",
		"tmux attach-session -E -t myapp-root",
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestEnterRootWorkspaceはPopup起動用にProjectRootを引き回す(t *testing.T) {
	t.Setenv("TMUX", "")
	runner := &fakeRunner{
		errors: map[string]error{
			"tmux has-session -t myapp-root": errors.New("exit status 1: can't find session: myapp-root"),
		},
	}
	adapter := TmuxAdapter{Runner: runner, Root: "/project"}

	if err := adapter.EnterRootWorkspace(context.Background(), "myapp"); err != nil {
		t.Fatalf("EnterRootWorkspaceでエラーが返った: %v", err)
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
		"tmux set-window-option -t myapp-root window-status-format #W#{?window_flags,#{window_flags}, }",
		"tmux set-window-option -t myapp-root window-status-current-format #W#{?window_flags,#{window_flags}, }",
		"tmux set-hook -t myapp-root after-new-window[100] set-window-option window-status-format '#W#{?window_flags,#{window_flags}, }'; set-window-option window-status-current-format '#W#{?window_flags,#{window_flags}, }'",
		"tmux set-option -t myapp-root key-table paracell-myapp-root",
		"tmux set-option -t myapp-root mouse on",
		"tmux set-option -t myapp-root set-clipboard on",
		"tmux bind-key -T paracell-myapp-root MouseDown1Pane select-pane -t = \\; send-keys -M",
		"tmux bind-key -T paracell-myapp-root MouseDrag1Pane if-shell -F #{||:#{pane_in_mode},#{mouse_any_flag}} send-keys -M copy-mode -M",
		"tmux bind-key -T paracell-myapp-root WheelUpPane if-shell -F #{||:#{alternate_on},#{pane_in_mode},#{mouse_any_flag}} send-keys -M copy-mode -e",
		"tmux bind-key -T paracell-myapp-root C-t next-window",
		"tmux bind-key -T paracell-myapp-root C-p display-popup -t myapp-root -w 65 -h 24 -d /project -E paracell view",
		"tmux attach-session -E -t myapp-root",
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestEnterRootWorkspaceはHasWorkspaceがexitStatus1だけでも作成してAttachする(t *testing.T) {
	t.Setenv("TMUX", "")
	runner := &fakeRunner{
		errors: map[string]error{
			"tmux has-session -t myapp-root": &exec.ExitError{},
		},
	}
	adapter := TmuxAdapter{Runner: runner}

	if err := adapter.EnterRootWorkspace(context.Background(), "myapp"); err != nil {
		t.Fatalf("EnterRootWorkspaceでエラーが返った: %v", err)
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
		"tmux set-window-option -t myapp-root window-status-format #W#{?window_flags,#{window_flags}, }",
		"tmux set-window-option -t myapp-root window-status-current-format #W#{?window_flags,#{window_flags}, }",
		"tmux set-hook -t myapp-root after-new-window[100] set-window-option window-status-format '#W#{?window_flags,#{window_flags}, }'; set-window-option window-status-current-format '#W#{?window_flags,#{window_flags}, }'",
		"tmux set-option -t myapp-root key-table paracell-myapp-root",
		"tmux set-option -t myapp-root mouse on",
		"tmux set-option -t myapp-root set-clipboard on",
		"tmux bind-key -T paracell-myapp-root MouseDown1Pane select-pane -t = \\; send-keys -M",
		"tmux bind-key -T paracell-myapp-root MouseDrag1Pane if-shell -F #{||:#{pane_in_mode},#{mouse_any_flag}} send-keys -M copy-mode -M",
		"tmux bind-key -T paracell-myapp-root WheelUpPane if-shell -F #{||:#{alternate_on},#{pane_in_mode},#{mouse_any_flag}} send-keys -M copy-mode -e",
		"tmux bind-key -T paracell-myapp-root C-t next-window",
		"tmux bind-key -T paracell-myapp-root C-p display-popup -t myapp-root -w 65 -h 24 -E paracell view",
		"tmux attach-session -E -t myapp-root",
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestEnterRootWorkspaceは既存WorkspaceでもPopupBindingを更新する(t *testing.T) {
	t.Setenv("TMUX", "")
	runner := &fakeRunner{}
	adapter := TmuxAdapter{Runner: runner, Root: "/project"}

	if err := adapter.EnterRootWorkspace(context.Background(), "myapp"); err != nil {
		t.Fatalf("EnterRootWorkspaceでエラーが返った: %v", err)
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
		"tmux set-window-option -t myapp-root window-status-format #W#{?window_flags,#{window_flags}, }",
		"tmux set-window-option -t myapp-root window-status-current-format #W#{?window_flags,#{window_flags}, }",
		"tmux set-hook -t myapp-root after-new-window[100] set-window-option window-status-format '#W#{?window_flags,#{window_flags}, }'; set-window-option window-status-current-format '#W#{?window_flags,#{window_flags}, }'",
		"tmux set-option -t myapp-root key-table paracell-myapp-root",
		"tmux set-option -t myapp-root mouse on",
		"tmux set-option -t myapp-root set-clipboard on",
		"tmux bind-key -T paracell-myapp-root MouseDown1Pane select-pane -t = \\; send-keys -M",
		"tmux bind-key -T paracell-myapp-root MouseDrag1Pane if-shell -F #{||:#{pane_in_mode},#{mouse_any_flag}} send-keys -M copy-mode -M",
		"tmux bind-key -T paracell-myapp-root WheelUpPane if-shell -F #{||:#{alternate_on},#{pane_in_mode},#{mouse_any_flag}} send-keys -M copy-mode -e",
		"tmux bind-key -T paracell-myapp-root C-t next-window",
		"tmux bind-key -T paracell-myapp-root C-p display-popup -t myapp-root -w 65 -h 24 -d /project -E paracell view",
		"tmux attach-session -E -t myapp-root",
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestEnterRootWorkspaceはTMUX内なら環境を更新せずswitchClientを使う(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-1000/default,123,0")
	runner := &fakeRunner{}
	adapter := TmuxAdapter{Runner: runner}

	if err := adapter.EnterRootWorkspace(context.Background(), "myapp"); err != nil {
		t.Fatalf("EnterRootWorkspaceでエラーが返った: %v", err)
	}
	if got := runner.calls[len(runner.calls)-1]; got != "tmux switch-client -E -t myapp-root" {
		t.Fatalf("last call = %q, want %q", got, "tmux switch-client -E -t myapp-root")
	}
}

func TestCreateWorkspaceはWindow未指定ならWorkspaceだけ作る(t *testing.T) {
	runner := &fakeRunner{}
	adapter := TmuxAdapter{Runner: runner, Root: "/project"}
	cell := namedWorkspaceResource("123", nil)

	if err := adapter.CreateWorkspace(context.Background(), cell); err != nil {
		t.Fatalf("CreateWorkspaceでエラーが返った: %v", err)
	}
	want := []string{
		"tmux -L paracell-paracell-myapp new-session -d -s paracell-myapp-123 -e PARACELL_CELL=123 -e PARACELL_ROOT=/project -c .paracell/cells/123/source",
		"tmux -L paracell-paracell-myapp set-option -t paracell-myapp-123 @paracell-project paracell-myapp",
		"tmux -L paracell-paracell-myapp set-option -t paracell-myapp-123 @paracell-status-label 123",
		"tmux -L paracell-paracell-myapp set-option -t paracell-myapp-123 set-titles on",
		"tmux -L paracell-paracell-myapp set-option -t paracell-myapp-123 set-titles-string #{@paracell-project}",
		"tmux -L paracell-paracell-myapp set-option -t paracell-myapp-123 status-left #{@paracell-status-label} ",
		"tmux -L paracell-paracell-myapp set-option -t paracell-myapp-123 status-left-length 100",
		"tmux -L paracell-paracell-myapp set-option -t paracell-myapp-123 status-right #{?window_bigger,[#{window_offset_x}#,#{window_offset_y}] ,}%H:%M %d-%b-%y",
		"tmux -L paracell-paracell-myapp set-window-option -t paracell-myapp-123 window-status-format #W#{?window_flags,#{window_flags}, }",
		"tmux -L paracell-paracell-myapp set-window-option -t paracell-myapp-123 window-status-current-format #W#{?window_flags,#{window_flags}, }",
		"tmux -L paracell-paracell-myapp set-hook -t paracell-myapp-123 after-new-window[100] set-window-option window-status-format '#W#{?window_flags,#{window_flags}, }'; set-window-option window-status-current-format '#W#{?window_flags,#{window_flags}, }'",
		"tmux -L paracell-paracell-myapp set-option -t paracell-myapp-123 key-table paracell-paracell-myapp-123",
		"tmux -L paracell-paracell-myapp set-option -t paracell-myapp-123 mouse on",
		"tmux -L paracell-paracell-myapp set-option -t paracell-myapp-123 set-clipboard on",
		"tmux -L paracell-paracell-myapp bind-key -T paracell-paracell-myapp-123 MouseDown1Pane select-pane -t = \\; send-keys -M",
		"tmux -L paracell-paracell-myapp bind-key -T paracell-paracell-myapp-123 MouseDrag1Pane if-shell -F #{||:#{pane_in_mode},#{mouse_any_flag}} send-keys -M copy-mode -M",
		"tmux -L paracell-paracell-myapp bind-key -T paracell-paracell-myapp-123 WheelUpPane if-shell -F #{||:#{alternate_on},#{pane_in_mode},#{mouse_any_flag}} send-keys -M copy-mode -e",
		"tmux -L paracell-paracell-myapp bind-key -T paracell-paracell-myapp-123 C-t next-window",
		"tmux -L paracell-paracell-myapp bind-key -T paracell-paracell-myapp-123 C-p display-popup -t paracell-myapp-123 -w 65 -h 24 -d /project -E paracell view",
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestCreateWorkspaceは指定Windowを作る(t *testing.T) {
	runner := &fakeRunner{}
	adapter := TmuxAdapter{Runner: runner, Root: "/project"}
	cell := namedWorkspaceResource("123", []domain.WorkspaceWindow{
		{Name: "123:editor"},
		{Name: "123:server"},
	})

	if err := adapter.CreateWorkspace(context.Background(), cell); err != nil {
		t.Fatalf("CreateWorkspaceでエラーが返った: %v", err)
	}
	want := []string{
		"tmux -L paracell-paracell-myapp new-session -d -s paracell-myapp-123 -e PARACELL_CELL=123 -e PARACELL_ROOT=/project -n editor -c .paracell/cells/123/source",
		"tmux -L paracell-paracell-myapp new-window -t paracell-myapp-123 -n server -c .paracell/cells/123/source",
		"tmux -L paracell-paracell-myapp set-option -t paracell-myapp-123 @paracell-project paracell-myapp",
		"tmux -L paracell-paracell-myapp set-option -t paracell-myapp-123 @paracell-status-label 123",
		"tmux -L paracell-paracell-myapp set-option -t paracell-myapp-123 set-titles on",
		"tmux -L paracell-paracell-myapp set-option -t paracell-myapp-123 set-titles-string #{@paracell-project}",
		"tmux -L paracell-paracell-myapp set-option -t paracell-myapp-123 status-left #{@paracell-status-label} ",
		"tmux -L paracell-paracell-myapp set-option -t paracell-myapp-123 status-left-length 100",
		"tmux -L paracell-paracell-myapp set-option -t paracell-myapp-123 status-right #{?window_bigger,[#{window_offset_x}#,#{window_offset_y}] ,}%H:%M %d-%b-%y",
		"tmux -L paracell-paracell-myapp set-window-option -t paracell-myapp-123:editor window-status-format #W#{?window_flags,#{window_flags}, }",
		"tmux -L paracell-paracell-myapp set-window-option -t paracell-myapp-123:editor window-status-current-format #W#{?window_flags,#{window_flags}, }",
		"tmux -L paracell-paracell-myapp set-window-option -t paracell-myapp-123:server window-status-format #W#{?window_flags,#{window_flags}, }",
		"tmux -L paracell-paracell-myapp set-window-option -t paracell-myapp-123:server window-status-current-format #W#{?window_flags,#{window_flags}, }",
		"tmux -L paracell-paracell-myapp set-hook -t paracell-myapp-123 after-new-window[100] set-window-option window-status-format '#W#{?window_flags,#{window_flags}, }'; set-window-option window-status-current-format '#W#{?window_flags,#{window_flags}, }'",
		"tmux -L paracell-paracell-myapp set-option -t paracell-myapp-123 key-table paracell-paracell-myapp-123",
		"tmux -L paracell-paracell-myapp set-option -t paracell-myapp-123 mouse on",
		"tmux -L paracell-paracell-myapp set-option -t paracell-myapp-123 set-clipboard on",
		"tmux -L paracell-paracell-myapp bind-key -T paracell-paracell-myapp-123 MouseDown1Pane select-pane -t = \\; send-keys -M",
		"tmux -L paracell-paracell-myapp bind-key -T paracell-paracell-myapp-123 MouseDrag1Pane if-shell -F #{||:#{pane_in_mode},#{mouse_any_flag}} send-keys -M copy-mode -M",
		"tmux -L paracell-paracell-myapp bind-key -T paracell-paracell-myapp-123 WheelUpPane if-shell -F #{||:#{alternate_on},#{pane_in_mode},#{mouse_any_flag}} send-keys -M copy-mode -e",
		"tmux -L paracell-paracell-myapp bind-key -T paracell-paracell-myapp-123 C-t next-window",
		"tmux -L paracell-paracell-myapp bind-key -T paracell-paracell-myapp-123 C-p display-popup -t paracell-myapp-123 -w 65 -h 24 -d /project -E paracell view",
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestCreateWorkspaceはWindow作成後にCommandをEnterで実行する(t *testing.T) {
	runner := &fakeRunner{}
	adapter := TmuxAdapter{Runner: runner, Root: "/project"}
	cell := namedWorkspaceResource("123", []domain.WorkspaceWindow{
		{Name: "123:editor", Command: "nvim ."},
		{Name: "server"},
		{Name: "123:test", Command: "go test ./..."},
	})

	if err := adapter.CreateWorkspace(context.Background(), cell); err != nil {
		t.Fatalf("CreateWorkspaceでエラーが返った: %v", err)
	}
	want := []string{
		"tmux -L paracell-paracell-myapp new-session -d -s paracell-myapp-123 -e PARACELL_CELL=123 -e PARACELL_ROOT=/project -n editor -c .paracell/cells/123/source",
		"tmux -L paracell-paracell-myapp send-keys -t paracell-myapp-123:editor nvim . Enter",
		"tmux -L paracell-paracell-myapp new-window -t paracell-myapp-123 -n server -c .paracell/cells/123/source",
		"tmux -L paracell-paracell-myapp new-window -t paracell-myapp-123 -n test -c .paracell/cells/123/source",
		"tmux -L paracell-paracell-myapp send-keys -t paracell-myapp-123:test go test ./... Enter",
		"tmux -L paracell-paracell-myapp set-option -t paracell-myapp-123 @paracell-project paracell-myapp",
		"tmux -L paracell-paracell-myapp set-option -t paracell-myapp-123 @paracell-status-label 123",
		"tmux -L paracell-paracell-myapp set-option -t paracell-myapp-123 set-titles on",
		"tmux -L paracell-paracell-myapp set-option -t paracell-myapp-123 set-titles-string #{@paracell-project}",
		"tmux -L paracell-paracell-myapp set-option -t paracell-myapp-123 status-left #{@paracell-status-label} ",
		"tmux -L paracell-paracell-myapp set-option -t paracell-myapp-123 status-left-length 100",
		"tmux -L paracell-paracell-myapp set-option -t paracell-myapp-123 status-right #{?window_bigger,[#{window_offset_x}#,#{window_offset_y}] ,}%H:%M %d-%b-%y",
		"tmux -L paracell-paracell-myapp set-window-option -t paracell-myapp-123:editor window-status-format #W#{?window_flags,#{window_flags}, }",
		"tmux -L paracell-paracell-myapp set-window-option -t paracell-myapp-123:editor window-status-current-format #W#{?window_flags,#{window_flags}, }",
		"tmux -L paracell-paracell-myapp set-window-option -t paracell-myapp-123:server window-status-format #W#{?window_flags,#{window_flags}, }",
		"tmux -L paracell-paracell-myapp set-window-option -t paracell-myapp-123:server window-status-current-format #W#{?window_flags,#{window_flags}, }",
		"tmux -L paracell-paracell-myapp set-window-option -t paracell-myapp-123:test window-status-format #W#{?window_flags,#{window_flags}, }",
		"tmux -L paracell-paracell-myapp set-window-option -t paracell-myapp-123:test window-status-current-format #W#{?window_flags,#{window_flags}, }",
		"tmux -L paracell-paracell-myapp set-hook -t paracell-myapp-123 after-new-window[100] set-window-option window-status-format '#W#{?window_flags,#{window_flags}, }'; set-window-option window-status-current-format '#W#{?window_flags,#{window_flags}, }'",
		"tmux -L paracell-paracell-myapp set-option -t paracell-myapp-123 key-table paracell-paracell-myapp-123",
		"tmux -L paracell-paracell-myapp set-option -t paracell-myapp-123 mouse on",
		"tmux -L paracell-paracell-myapp set-option -t paracell-myapp-123 set-clipboard on",
		"tmux -L paracell-paracell-myapp bind-key -T paracell-paracell-myapp-123 MouseDown1Pane select-pane -t = \\; send-keys -M",
		"tmux -L paracell-paracell-myapp bind-key -T paracell-paracell-myapp-123 MouseDrag1Pane if-shell -F #{||:#{pane_in_mode},#{mouse_any_flag}} send-keys -M copy-mode -M",
		"tmux -L paracell-paracell-myapp bind-key -T paracell-paracell-myapp-123 WheelUpPane if-shell -F #{||:#{alternate_on},#{pane_in_mode},#{mouse_any_flag}} send-keys -M copy-mode -e",
		"tmux -L paracell-paracell-myapp bind-key -T paracell-paracell-myapp-123 C-t next-window",
		"tmux -L paracell-paracell-myapp bind-key -T paracell-paracell-myapp-123 C-p display-popup -t paracell-myapp-123 -w 65 -h 24 -d /project -E paracell view",
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestCreateWorkspaceは複数IssueのPopupBindingをWorkspaceごとに分離する(t *testing.T) {
	runner := &fakeRunner{}
	adapter := TmuxAdapter{Runner: runner, Root: "/project"}
	cells := []domain.WorkspaceResource{
		namedWorkspaceResource("123", nil),
		namedWorkspaceResource("456", nil),
	}

	for _, cell := range cells {
		if err := adapter.CreateWorkspace(context.Background(), cell); err != nil {
			t.Fatalf("CreateWorkspace(%s)でエラーが返った: %v", cell.Name, err)
		}
	}

	wantCalls := []string{
		"tmux -L paracell-paracell-myapp set-option -t paracell-myapp-123 key-table paracell-paracell-myapp-123",
		"tmux -L paracell-paracell-myapp bind-key -T paracell-paracell-myapp-123 C-p display-popup -t paracell-myapp-123 -w 65 -h 24 -d /project -E paracell view",
		"tmux -L paracell-paracell-myapp set-option -t paracell-myapp-456 key-table paracell-paracell-myapp-456",
		"tmux -L paracell-paracell-myapp bind-key -T paracell-paracell-myapp-456 C-p display-popup -t paracell-myapp-456 -w 65 -h 24 -d /project -E paracell view",
	}
	for _, want := range wantCalls {
		if !containsCall(runner.calls, want) {
			t.Errorf("callsに %q がない: %#v", want, runner.calls)
		}
	}
}

func TestCleanWorkspaceは見つからないWorkspaceをnotFound扱いにする(t *testing.T) {
	runner := &fakeRunner{
		errors: map[string]error{
			"tmux -L paracell-paracell-myapp kill-session -t paracell-myapp-123": errors.New("exit status 1: can't find session: paracell-myapp-123"),
		},
	}
	adapter := TmuxAdapter{Runner: runner}
	cell := namedWorkspaceResource("123", nil)

	err := adapter.CleanWorkspace(context.Background(), cell)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

const (
	dedicatedSocket = "paracell-paracell-myapp"
	cellHasSession  = "tmux -L " + dedicatedSocket + " has-session -t =paracell-myapp-123"
	legacyHasCell   = "tmux has-session -t =paracell-myapp-123"
)

func legacyCellRunner() *fakeRunner {
	return &fakeRunner{errors: map[string]error{
		cellHasSession: errors.New("exit status 1: no server running"),
	}}
}

func TestCreateWorkspaceは専用serverだけに作りdefaultServerを触らない(t *testing.T) {
	runner := &fakeRunner{}
	adapter := TmuxAdapter{Runner: runner, Root: "/project"}
	if err := adapter.CreateWorkspace(context.Background(), namedWorkspaceResource("123", []domain.WorkspaceWindow{{Name: "a", Command: "ls"}, {Name: "b"}})); err != nil {
		t.Fatal(err)
	}
	for _, call := range append(append([]string(nil), runner.calls...), runner.environmentCalls...) {
		if !strings.HasPrefix(call, "tmux -L "+dedicatedSocket+" ") {
			t.Fatalf("専用server以外へのtmux呼び出し: %q", call)
		}
	}
}

func TestCreateWorkspaceの失敗時cleanupは専用serverのsessionを削除する(t *testing.T) {
	runner := &fakeRunner{errors: map[string]error{
		"tmux -L " + dedicatedSocket + " set-option -t paracell-myapp-123 mouse on": errors.New("boom"),
	}}
	if err := (TmuxAdapter{Runner: runner}).CreateWorkspace(context.Background(), namedWorkspaceResource("123", nil)); err == nil {
		t.Fatal("エラーが返らなかった")
	}
	if !containsCall(runner.calls, "tmux -L "+dedicatedSocket+" kill-session -t paracell-myapp-123") {
		t.Fatalf("calls = %#v", runner.calls)
	}
}

func TestCleanWorkspaceは専用serverのsessionをkillする(t *testing.T) {
	runner := &fakeRunner{}
	if err := (TmuxAdapter{Runner: runner}).CleanWorkspace(context.Background(), namedWorkspaceResource("123", nil)); err != nil {
		t.Fatal(err)
	}
	want := []string{cellHasSession, "tmux -L " + dedicatedSocket + " kill-session -t paracell-myapp-123"}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestCleanWorkspaceは専用serverにない既存cellをdefaultServerでkillする(t *testing.T) {
	runner := legacyCellRunner()
	if err := (TmuxAdapter{Runner: runner}).CleanWorkspace(context.Background(), namedWorkspaceResource("123", nil)); err != nil {
		t.Fatal(err)
	}
	want := []string{cellHasSession, legacyHasCell, "tmux kill-session -t paracell-myapp-123"}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestCleanWorkspaceはどのserverにもなければnotFound扱いにする(t *testing.T) {
	runner := legacyCellRunner()
	runner.errors[legacyHasCell] = errors.New("exit status 1")
	runner.errors["tmux -L "+dedicatedSocket+" kill-session -t paracell-myapp-123"] = errors.New("exit status 1: can't find session: paracell-myapp-123")
	err := (TmuxAdapter{Runner: runner}).CleanWorkspace(context.Background(), namedWorkspaceResource("123", nil))
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestUpdateStatusLabelは専用serverにない既存cellをdefaultServerで更新する(t *testing.T) {
	runner := legacyCellRunner()
	if err := (TmuxAdapter{Runner: runner}).UpdateStatusLabel(context.Background(), cellWorkspaceResource("note", nil)); err != nil {
		t.Fatal(err)
	}
	want := []string{cellHasSession, legacyHasCell, "tmux set-option -t paracell-myapp-123 @paracell-status-label note"}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func lastCall(runner *fakeRunner) string { return runner.calls[len(runner.calls)-1] }

func TestEnterWorkspaceの移動先はserverの組み合わせで決まる(t *testing.T) {
	const detachToCell = "tmux detach-client -E 'tmux' '-L' 'paracell-paracell-myapp' 'attach-session' '-E' '-t' 'paracell-myapp-123'"
	tests := []struct {
		name   string
		tmux   string
		runner *fakeRunner
		want   string
	}{
		{"tmux外は専用serverへattach", "", &fakeRunner{}, "tmux -L paracell-paracell-myapp attach-session -E -t paracell-myapp-123"},
		{"root(default server)から専用serverへdetachで付け替える", "/tmp/tmux-501/default,1,0", &fakeRunner{}, detachToCell},
		{"cell(専用server)からcellへswitch-client", "/tmp/tmux-501/paracell-paracell-myapp,1,0", &fakeRunner{}, "tmux -L paracell-paracell-myapp switch-client -E -t paracell-myapp-123"},
		{"別のserverからも専用serverへdetachで付け替える", "/tmp/tmux-501/work,1,0", &fakeRunner{}, detachToCell},
		{"既存cell(default)へdefaultからswitch-client", "/tmp/tmux-501/default,1,0", legacyCellRunner(), "tmux switch-client -E -t paracell-myapp-123"},
		{"既存cell(default)へ専用serverからdetachで付け替える", "/tmp/tmux-501/paracell-paracell-myapp,1,0", legacyCellRunner(), "tmux detach-client -E 'tmux' 'attach-session' '-E' '-t' 'paracell-myapp-123'"},
		{"既存cell(default)へtmux外からattach", "", legacyCellRunner(), "tmux attach-session -E -t paracell-myapp-123"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TMUX", tt.tmux)
			if err := (TmuxAdapter{Runner: tt.runner}).EnterWorkspace(context.Background(), cellWorkspaceResource("", nil)); err != nil {
				t.Fatal(err)
			}
			if got := lastCall(tt.runner); got != tt.want {
				t.Fatalf("last call = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEnterCommandはEnterWorkspaceと同じ引数のcommandを返す(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-501/default,1,0")
	runner := &fakeRunner{}
	cmd, err := (TmuxAdapter{Runner: runner}).EnterCommand(context.Background(), cellWorkspaceResource("", nil))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"tmux", "detach-client", "-E", "'tmux' '-L' 'paracell-paracell-myapp' 'attach-session' '-E' '-t' 'paracell-myapp-123'"}
	if !reflect.DeepEqual(cmd.Args, want) {
		t.Fatalf("args = %#v, want %#v", cmd.Args, want)
	}
	if !containsCall(runner.calls, "tmux -L "+dedicatedSocket+" set-option -t paracell-myapp-123 mouse on") {
		t.Fatalf("cell session was not prepared on the dedicated server: %#v", runner.calls)
	}
}

func TestEnterRootWorkspaceの移動先はserverで決まる(t *testing.T) {
	tests := []struct {
		name string
		tmux string
		want string
	}{
		{"tmux外はattach", "", "tmux attach-session -E -t myapp-root"},
		{"default serverからはswitch-client", "/tmp/tmux-501/default,1,0", "tmux switch-client -E -t myapp-root"},
		{"cell(専用server)からはdetachでdefault serverへ付け替える", "/tmp/tmux-501/paracell-myapp,1,0", "tmux detach-client -E 'tmux' 'attach-session' '-E' '-t' 'myapp-root'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TMUX", tt.tmux)
			runner := &fakeRunner{}
			if err := (TmuxAdapter{Runner: runner}).EnterRootWorkspace(context.Background(), "myapp"); err != nil {
				t.Fatal(err)
			}
			if got := lastCall(runner); got != tt.want {
				t.Fatalf("last call = %q, want %q", got, tt.want)
			}
			for _, call := range runner.calls {
				if strings.Contains(call, " -L ") {
					t.Fatalf("root session must stay on the default server: %q", call)
				}
			}
		})
	}
}

func TestExitWorkspaceは現在のserverのclientをdetachする(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-501/paracell-paracell-myapp,1,0")
	runner := &fakeRunner{}
	if err := (TmuxAdapter{Runner: runner}).ExitWorkspace(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(runner.calls, []string{"tmux detach-client"}) {
		t.Fatalf("calls = %#v", runner.calls)
	}
}

func TestShellQuoteはsingleQuoteを含む名前を安全に引用する(t *testing.T) {
	if got := shellQuote("a'b c"); got != `'a'\''b c'` {
		t.Fatalf("shellQuote = %s", got)
	}
}
