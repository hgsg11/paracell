package source

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateSourceは複数RepositoryのWorktreeを作る(t *testing.T) {
	runner := &fakeRunner{}
	for _, path := range []string{".", "api"} {
		base := "main"
		worktree := filepath.Join(".paracell/cells/42/source", path)
		if err := (GitSourceAdapter{Runner: runner, Root: "/project"}).CreateSource(context.Background(), path, worktree, base, "feat/42"); err != nil {
			t.Fatal(err)
		}
	}
	if got := strings.Join(runner.runCalls, "\n"); !strings.Contains(got, "git -C /project/api worktree add /project/.paracell/cells/42/source/api -b feat/42 main") {
		t.Fatalf("calls = %s", got)
	}
}

type fakeRunner struct {
	runCalls []string
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) error {
	call := strings.Join(append([]string{name}, args...), " ")
	f.runCalls = append(f.runCalls, call)
	return nil
}

func (f *fakeRunner) Output(_ context.Context, name string, args ...string) (string, error) {
	return "", f.Run(context.Background(), name, args...)
}
