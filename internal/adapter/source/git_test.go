package source

import (
	"context"
	"errors"
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
		Base:   "feature/111",
		Branch: "feat/123",
		Source: domain.Source{Path: ".paracell/cells/123/source"},
	}

	if err := adapter.CreateSource(context.Background(), cell); err != nil {
		t.Fatalf("CreateSourceでエラーが返った: %v", err)
	}

	want := []string{
		"git worktree add .paracell/cells/123/source -b feat/123 feature/111",
	}
	if !reflect.DeepEqual(runner.runCalls, want) {
		t.Fatalf("run calls = %#v, want %#v", runner.runCalls, want)
	}
}

func TestCreateSourceはBranchModeReuseで既存BranchならWorktreeを切り替える(t *testing.T) {
	runner := &fakeRunner{}
	adapter := GitSourceAdapter{Runner: runner}
	cell := domain.Cell{
		Base:       "main",
		Branch:     "feat/123",
		BranchMode: "reuse",
		Source:     domain.Source{Path: ".paracell/cells/123/source"},
	}

	if err := adapter.CreateSource(context.Background(), cell); err != nil {
		t.Fatalf("CreateSourceでエラーが返った: %v", err)
	}

	want := []string{
		"git show-ref --verify --quiet refs/heads/feat/123",
		"git worktree add .paracell/cells/123/source feat/123",
	}
	if !reflect.DeepEqual(runner.runCalls, want) {
		t.Fatalf("run calls = %#v, want %#v", runner.runCalls, want)
	}
}

func TestCreateSourceはBranchModeReuseでBranchがなければ作成する(t *testing.T) {
	runner := &fakeRunner{
		runErrors: map[string]error{
			"git show-ref --verify --quiet refs/heads/feat/123": errors.New("not found"),
		},
	}
	adapter := GitSourceAdapter{Runner: runner}
	cell := domain.Cell{
		Base:       "main",
		Branch:     "feat/123",
		BranchMode: "reuse",
		Source:     domain.Source{Path: ".paracell/cells/123/source"},
	}

	if err := adapter.CreateSource(context.Background(), cell); err != nil {
		t.Fatalf("CreateSourceでエラーが返った: %v", err)
	}

	want := []string{
		"git show-ref --verify --quiet refs/heads/feat/123",
		"git worktree add .paracell/cells/123/source -b feat/123 main",
	}
	if !reflect.DeepEqual(runner.runCalls, want) {
		t.Fatalf("run calls = %#v, want %#v", runner.runCalls, want)
	}
}

func TestCreateSourceはBranchModeRequireで既存Branchを使う(t *testing.T) {
	runner := &fakeRunner{}
	adapter := GitSourceAdapter{Runner: runner}
	cell := domain.Cell{
		Base:       "main",
		Branch:     "feat/123",
		BranchMode: "require",
		Source:     domain.Source{Path: ".paracell/cells/123/source"},
	}

	if err := adapter.CreateSource(context.Background(), cell); err != nil {
		t.Fatalf("CreateSourceでエラーが返った: %v", err)
	}

	want := []string{
		"git worktree add .paracell/cells/123/source feat/123",
	}
	if !reflect.DeepEqual(runner.runCalls, want) {
		t.Fatalf("run calls = %#v, want %#v", runner.runCalls, want)
	}
}

func TestCleanSourceは見つからないWorktreeをnotFound扱いにする(t *testing.T) {
	runner := &fakeRunner{
		runErrors: map[string]error{
			"git worktree remove --force .paracell/cells/123/source": errors.New("fatal: '.paracell/cells/123/source' is not a working tree"),
		},
	}
	adapter := GitSourceAdapter{Runner: runner}
	cell := domain.Cell{
		Source: domain.Source{Path: ".paracell/cells/123/source"},
	}

	err := adapter.CleanSource(context.Background(), cell)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want domain.ErrNotFound", err)
	}
}

type fakeRunner struct {
	runCalls  []string
	runErrors map[string]error
}

func (r *fakeRunner) Run(ctx context.Context, name string, args ...string) error {
	_ = ctx
	call := name + " " + joinArgs(args)
	r.runCalls = append(r.runCalls, call)
	if r.runErrors != nil && r.runErrors[call] != nil {
		return r.runErrors[call]
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
