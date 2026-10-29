package source

import (
	"context"
	"errors"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/hgsg11/paracell/internal/domain"
)

func TestCreateSourceは複数RepositoryのWorktreeを作る(t *testing.T) {
	runner := newFakeRunner()
	resources := []domain.SourceResource{
		domain.NewSourceResource(".", ".paracell/cells/42/source", "main", "feat/42"),
		domain.NewSourceResource("api", ".paracell/cells/42/source/api", "main", "feat/42"),
	}
	for _, resource := range resources {
		if err := (GitSourceAdapter{Runner: runner, Root: "/project"}).CreateSource(context.Background(), resource); err != nil {
			t.Fatal(err)
		}
	}
	if got := strings.Join(runner.runCalls, "\n"); !strings.Contains(got, "git -C /project/api worktree add -b feat/42 /project/.paracell/cells/42/source/api main") {
		t.Fatalf("calls = %s", got)
	}
}

func TestCreateSourceはOriginがあればWorktree作成前にFetchする(t *testing.T) {
	runner := newFakeRunner()
	runner.outputs["git -C /project remote"] = "upstream\norigin"

	if err := (GitSourceAdapter{Runner: runner, Root: "/project"}).CreateSource(context.Background(), domain.NewSourceResource(".", "cell", "origin/main", "feat/42")); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"git -C /project remote",
		"git -C /project fetch origin",
		"git -C /project show-ref --verify --quiet refs/heads/feat/42",
		"git -C /project show-ref --verify --quiet refs/remotes/origin/feat/42",
		"git -C /project worktree add -b feat/42 /project/cell origin/main",
	}
	assertCalls(t, runner.runCalls, want)
}

func TestCreateSourceはOriginがなければFetchしない(t *testing.T) {
	runner := newFakeRunner()
	runner.outputs["git -C /project remote"] = "upstream"

	if err := (GitSourceAdapter{Runner: runner, Root: "/project"}).CreateSource(context.Background(), domain.NewSourceResource(".", "cell", "main", "feat/42")); err != nil {
		t.Fatal(err)
	}
	for _, call := range runner.runCalls {
		if strings.Contains(call, "fetch") {
			t.Fatalf("calls = %v", runner.runCalls)
		}
	}
}

func TestCreateSourceはFetchが失敗したらWorktreeを作らずエラーを返す(t *testing.T) {
	runner := newFakeRunner()
	runner.outputs["git -C /project remote"] = "origin"
	fetchErr := errors.New("could not read from remote repository")
	runner.errors["git -C /project fetch origin"] = fetchErr

	err := (GitSourceAdapter{Runner: runner, Root: "/project"}).CreateSource(context.Background(), domain.NewSourceResource(".", "cell", "main", "feat/42"))
	if !errors.Is(err, fetchErr) {
		t.Fatalf("err = %v", err)
	}
	for _, call := range runner.runCalls {
		if strings.Contains(call, "worktree add") {
			t.Fatalf("calls = %v", runner.runCalls)
		}
	}
}

func TestCreateSourceはローカルBranchがあればそのBranchのWorktreeを作る(t *testing.T) {
	runner := newFakeRunner()
	runner.found["refs/heads/feat/42"] = true
	runner.found["refs/remotes/origin/feat/42"] = true

	if err := (GitSourceAdapter{Runner: runner, Root: "/project"}).CreateSource(context.Background(), domain.NewSourceResource(".", "cell", "main", "feat/42")); err != nil {
		t.Fatal(err)
	}
	assertLastCall(t, runner.runCalls, "git -C /project worktree add /project/cell feat/42")
}

func TestCreateSourceはOriginにだけBranchがあれば追跡Branchを作る(t *testing.T) {
	runner := newFakeRunner()
	runner.found["refs/remotes/origin/feat/42"] = true

	if err := (GitSourceAdapter{Runner: runner, Root: "/project"}).CreateSource(context.Background(), domain.NewSourceResource(".", "cell", "main", "feat/42")); err != nil {
		t.Fatal(err)
	}
	assertLastCall(t, runner.runCalls, "git -C /project worktree add --track -b feat/42 /project/cell origin/feat/42")
}

func TestCreateSourceはBranchがなければBaseから作る(t *testing.T) {
	for _, tt := range []struct {
		base string
		want string
	}{
		{base: "main", want: "git -C /project worktree add -b feat/42 /project/cell main"},
		{base: "origin/main", want: "git -C /project worktree add -b feat/42 /project/cell origin/main"},
		{base: "current", want: "git -C /project worktree add -b feat/42 /project/cell"},
		{base: "", want: "git -C /project worktree add -b feat/42 /project/cell"},
	} {
		t.Run(tt.base, func(t *testing.T) {
			runner := newFakeRunner()
			if err := (GitSourceAdapter{Runner: runner, Root: "/project"}).CreateSource(context.Background(), domain.NewSourceResource(".", "cell", tt.base, "feat/42")); err != nil {
				t.Fatal(err)
			}
			assertLastCall(t, runner.runCalls, tt.want)
		})
	}
}

func TestCreateSourceはBranchの確認に失敗したらエラーを返す(t *testing.T) {
	runner := newFakeRunner()
	showRefErr := exitError(t, 128)
	runner.errors["git -C /project show-ref --verify --quiet refs/heads/feat/42"] = showRefErr

	err := (GitSourceAdapter{Runner: runner, Root: "/project"}).CreateSource(context.Background(), domain.NewSourceResource(".", "cell", "main", "feat/42"))
	if !errors.Is(err, showRefErr) {
		t.Fatalf("err = %v", err)
	}
}

func TestCreateSourceは使用中BranchのWorktree作成エラーをそのまま返す(t *testing.T) {
	runner := newFakeRunner()
	runner.found["refs/heads/feat/42"] = true
	addErr := errors.New("fatal: 'feat/42' is already used by worktree at '/project'")
	runner.errors["git -C /project worktree add /project/cell feat/42"] = addErr

	err := (GitSourceAdapter{Runner: runner, Root: "/project"}).CreateSource(context.Background(), domain.NewSourceResource(".", "cell", "main", "feat/42"))
	if !errors.Is(err, addErr) {
		t.Fatalf("err = %v", err)
	}
}

func assertCalls(t *testing.T, got, want []string) {
	t.Helper()
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("calls =\n%s\nwant =\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func assertLastCall(t *testing.T, calls []string, want string) {
	t.Helper()
	if len(calls) == 0 || calls[len(calls)-1] != want {
		t.Fatalf("calls = %v, want last %q", calls, want)
	}
}

// exitError returns a real *exec.ExitError so ExitCode reflects the given status.
func exitError(t *testing.T, code int) error {
	t.Helper()
	err := exec.Command("sh", "-c", "exit "+strconv.Itoa(code)).Run()
	if err == nil {
		t.Fatal("expected exit error")
	}
	return err
}

type fakeRunner struct {
	runCalls []string
	outputs  map[string]string
	errors   map[string]error
	// found lists refs that show-ref reports as existing; missing refs exit with status 1.
	found map[string]bool
	// missingRef is the exit status 1 error show-ref returns for a missing ref.
	missingRef error
}

func newFakeRunner() *fakeRunner {
	return &fakeRunner{outputs: map[string]string{}, errors: map[string]error{}, found: map[string]bool{}}
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) error {
	call := strings.Join(append([]string{name}, args...), " ")
	f.runCalls = append(f.runCalls, call)
	if err, ok := f.errors[call]; ok {
		return err
	}
	if len(args) > 0 && strings.Contains(call, " show-ref --verify --quiet ") && !f.found[args[len(args)-1]] {
		if f.missingRef == nil {
			f.missingRef = exec.Command("sh", "-c", "exit 1").Run()
		}
		return f.missingRef
	}
	return nil
}

func (f *fakeRunner) Output(_ context.Context, name string, args ...string) (string, error) {
	call := strings.Join(append([]string{name}, args...), " ")
	if err := f.Run(context.Background(), name, args...); err != nil {
		return "", err
	}
	return f.outputs[call], nil
}
