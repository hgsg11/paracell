package source

import (
	"context"
	"strings"
	"testing"

	"github.com/hgsg11/paracell/internal/domain"
)

func TestCreateSourceは複数RepositoryのWorktreeを作る(t *testing.T) {
	runner := &fakeRunner{runErrors: map[string]error{
		"git -C /project show-ref --verify --quiet refs/heads/feat/42":     exitCodeError{code: 1},
		"git -C /project/api show-ref --verify --quiet refs/heads/feat/42": exitCodeError{code: 1},
	}}
	cell := domain.Cell{Sources: []domain.Source{
		{TemplatePath: ".", Path: ".paracell/cells/42/source", Base: "main", Branch: "feat/42"},
		{TemplatePath: "api", Path: ".paracell/cells/42/source/api", Base: "main", Branch: "feat/42"},
	}}
	if _, err := (GitSourceAdapter{Runner: runner, Root: "/project"}).CreateSource(context.Background(), cell); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(runner.runCalls, "\n"); !strings.Contains(got, "git -C /project/api worktree add /project/.paracell/cells/42/source/api -b feat/42 main") {
		t.Fatalf("calls = %s", got)
	}
}

type fakeRunner struct {
	runCalls  []string
	runErrors map[string]error
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) error {
	call := strings.Join(append([]string{name}, args...), " ")
	f.runCalls = append(f.runCalls, call)
	return f.runErrors[call]
}

func (f *fakeRunner) Output(_ context.Context, name string, args ...string) (string, error) {
	return "", f.Run(context.Background(), name, args...)
}

type exitCodeError struct{ code int }

func (e exitCodeError) Error() string { return "exit" }
func (e exitCodeError) ExitCode() int { return e.code }
