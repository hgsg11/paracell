package source

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hgsg11/paracell/internal/domain"
)

func TestCreateSourceは複数RepositoryのWorktreeを作る(t *testing.T) {
	runner := &fakeRunner{}
	for _, path := range []string{".", "api"} {
		source, err := domain.NewSource(path, "main", "feat/42")
		if err != nil {
			t.Fatal(err)
		}
		worktree := filepath.Join(".paracell/cells/42/source", path)
		if err := (GitSourceAdapter{Runner: runner, Root: "/project"}).CreateSource(context.Background(), source, worktree); err != nil {
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
