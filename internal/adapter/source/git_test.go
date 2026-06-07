package source

import (
	"context"
	"reflect"
	"testing"

	"github.com/hgsg11/paracell/internal/domain"
)

func TestCreateSourceはBaseCurrentなら現在BranchからWorktreeを作る(t *testing.T) {
	runner := &fakeRunner{}
	adapter := GitSourceAdapter{Runner: runner}
	cell := domain.Cell{
		Base:   "current",
		Branch: "feat/123",
		Source: domain.Source{Path: ".paracell/cells/123/source"},
	}

	if err := adapter.CreateSource(context.Background(), cell); err != nil {
		t.Fatalf("CreateSourceでエラーが返った: %v", err)
	}

	want := []string{
		"git worktree add .paracell/cells/123/source -b feat/123",
	}
	if !reflect.DeepEqual(runner.runCalls, want) {
		t.Fatalf("run calls = %#v, want %#v", runner.runCalls, want)
	}
}

func TestCreateSourceはNamedBaseなら明示BaseからWorktreeを作る(t *testing.T) {
	runner := &fakeRunner{}
	adapter := GitSourceAdapter{Runner: runner}
	cell := domain.Cell{
		Base:   "main",
		Branch: "feat/123",
		Source: domain.Source{Path: ".paracell/cells/123/source"},
	}

	if err := adapter.CreateSource(context.Background(), cell); err != nil {
		t.Fatalf("CreateSourceでエラーが返った: %v", err)
	}

	want := []string{
		"git worktree add .paracell/cells/123/source -b feat/123 main",
	}
	if !reflect.DeepEqual(runner.runCalls, want) {
		t.Fatalf("run calls = %#v, want %#v", runner.runCalls, want)
	}
}

type fakeRunner struct {
	runCalls []string
}

func (r *fakeRunner) Run(ctx context.Context, name string, args ...string) error {
	_ = ctx
	r.runCalls = append(r.runCalls, name+" "+joinArgs(args))
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
