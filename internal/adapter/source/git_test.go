package source

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/hgsg11/paracell/internal/domain"
	"github.com/hgsg11/paracell/internal/usecase"
)

func TestCreateSourceはBaseCurrentなら現在BranchからWorktreeを作る(t *testing.T) {
	runner := &fakeRunner{runErrors: map[string]error{
		"git show-ref --verify --quiet refs/heads/feat/123": exitCodeError{code: 1},
	}}
	adapter := GitSourceAdapter{Runner: runner}
	cell := domain.Cell{
		Base:       "current",
		Branch:     "feat/123",
		BranchMode: domain.RepositoryBranchModeCreate,
		Source:     domain.Source{Path: ".paracell/cells/123/source"},
	}

	creation, err := adapter.CreateSource(context.Background(), cell)
	if err != nil {
		t.Fatalf("CreateSourceでエラーが返った: %v", err)
	}
	if !creation.BranchCreated {
		t.Fatal("BranchCreated = false, want true")
	}

	want := []string{
		"git show-ref --verify --quiet refs/heads/feat/123",
		"git worktree add .paracell/cells/123/source -b feat/123",
	}
	if !reflect.DeepEqual(runner.runCalls, want) {
		t.Fatalf("run calls = %#v, want %#v", runner.runCalls, want)
	}
}

func TestCreateSourceはNamedBaseなら明示BaseからWorktreeを作る(t *testing.T) {
	runner := &fakeRunner{runErrors: map[string]error{
		"git show-ref --verify --quiet refs/heads/feat/123": exitCodeError{code: 1},
	}}
	adapter := GitSourceAdapter{Runner: runner}
	cell := domain.Cell{
		Base:   "feature/111",
		Branch: "feat/123",
		Source: domain.Source{Path: ".paracell/cells/123/source"},
	}

	creation, err := adapter.CreateSource(context.Background(), cell)
	if err != nil {
		t.Fatalf("CreateSourceでエラーが返った: %v", err)
	}
	if !creation.BranchCreated {
		t.Fatal("BranchCreated = false, want true")
	}

	want := []string{
		"git show-ref --verify --quiet refs/heads/feat/123",
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

	creation, err := adapter.CreateSource(context.Background(), cell)
	if err != nil {
		t.Fatalf("CreateSourceでエラーが返った: %v", err)
	}
	if creation.BranchCreated {
		t.Fatal("BranchCreated = true, want false")
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
			"git show-ref --verify --quiet refs/heads/feat/123": exitCodeError{code: 1},
		},
	}
	adapter := GitSourceAdapter{Runner: runner}
	cell := domain.Cell{
		Base:       "main",
		Branch:     "feat/123",
		BranchMode: "reuse",
		Source:     domain.Source{Path: ".paracell/cells/123/source"},
	}

	creation, err := adapter.CreateSource(context.Background(), cell)
	if err != nil {
		t.Fatalf("CreateSourceでエラーが返った: %v", err)
	}
	if !creation.BranchCreated {
		t.Fatal("BranchCreated = false, want true")
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

	creation, err := adapter.CreateSource(context.Background(), cell)
	if err != nil {
		t.Fatalf("CreateSourceでエラーが返った: %v", err)
	}
	if creation.BranchCreated {
		t.Fatal("BranchCreated = true, want false")
	}

	want := []string{
		"git worktree add .paracell/cells/123/source feat/123",
	}
	if !reflect.DeepEqual(runner.runCalls, want) {
		t.Fatalf("run calls = %#v, want %#v", runner.runCalls, want)
	}
}

func TestCreateSourceはBranch確認失敗時に作成を開始しない(t *testing.T) {
	checkErr := errors.New("git unavailable")
	runner := &fakeRunner{runErrors: map[string]error{
		"git show-ref --verify --quiet refs/heads/feat/123": checkErr,
	}}
	adapter := GitSourceAdapter{Runner: runner}
	cell := domain.Cell{Branch: "feat/123", Source: domain.Source{Path: ".paracell/cells/123/source"}}

	creation, err := adapter.CreateSource(context.Background(), cell)

	if !errors.Is(err, checkErr) {
		t.Fatalf("error = %v, want check error", err)
	}
	if creation.BranchCreated {
		t.Fatal("BranchCreated = true, want false")
	}
	want := []string{"git show-ref --verify --quiet refs/heads/feat/123"}
	if !reflect.DeepEqual(runner.runCalls, want) {
		t.Fatalf("run calls = %#v, want %#v", runner.runCalls, want)
	}
}

func TestCreateSourceは失敗時に部分作成されたBranchを明示する(t *testing.T) {
	createErr := errors.New("worktree setup failed")
	checkCall := "git show-ref --verify --quiet refs/heads/feat/123"
	runner := &fakeRunner{
		runErrors: map[string]error{
			checkCall: exitCodeError{code: 1},
			"git worktree add .paracell/cells/123/source -b feat/123 main": createErr,
		},
		runErrorSequences: map[string][]error{
			checkCall: {exitCodeError{code: 1}, nil},
		},
	}
	adapter := GitSourceAdapter{Runner: runner}
	cell := domain.Cell{Base: "main", Branch: "feat/123", Source: domain.Source{Path: ".paracell/cells/123/source"}}

	creation, err := adapter.CreateSource(context.Background(), cell)

	if !errors.Is(err, createErr) {
		t.Fatalf("error = %v, want create error", err)
	}
	if !creation.BranchCreated {
		t.Fatal("BranchCreated = false, want true")
	}
}

func TestRollbackSourceは作成したBranchだけをWorktreeの後に削除する(t *testing.T) {
	tests := []struct {
		name          string
		branchCreated bool
		wantCalls     []string
	}{
		{
			name:          "新規branch",
			branchCreated: true,
			wantCalls: []string{
				"git worktree remove --force .paracell/cells/123/source",
				"git branch -D feat/123",
			},
		},
		{
			name:          "既存branchへattach",
			branchCreated: false,
			wantCalls: []string{
				"git worktree remove --force .paracell/cells/123/source",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeRunner{}
			adapter := GitSourceAdapter{Runner: runner}
			cell := domain.Cell{Branch: "feat/123", Source: domain.Source{Path: ".paracell/cells/123/source"}}

			if err := adapter.RollbackSource(context.Background(), cell, usecase.SourceCreation{BranchCreated: tt.branchCreated}); err != nil {
				t.Fatalf("RollbackSourceでエラーが返った: %v", err)
			}
			if !reflect.DeepEqual(runner.runCalls, tt.wantCalls) {
				t.Fatalf("run calls = %#v, want %#v", runner.runCalls, tt.wantCalls)
			}
		})
	}
}

func TestRollbackSourceはWorktree削除失敗後もBranchを削除してErrorを結合する(t *testing.T) {
	worktreeErr := errors.New("worktree remove failed")
	branchErr := errors.New("branch delete failed")
	runner := &fakeRunner{runErrors: map[string]error{
		"git worktree remove --force .paracell/cells/123/source": worktreeErr,
		"git branch -D feat/123":                                 branchErr,
	}}
	adapter := GitSourceAdapter{Runner: runner}
	cell := domain.Cell{Branch: "feat/123", Source: domain.Source{Path: ".paracell/cells/123/source"}}

	err := adapter.RollbackSource(context.Background(), cell, usecase.SourceCreation{BranchCreated: true})

	if !errors.Is(err, worktreeErr) || !errors.Is(err, branchErr) {
		t.Fatalf("error = %v, want both cleanup errors", err)
	}
	want := []string{
		"git worktree remove --force .paracell/cells/123/source",
		"git branch -D feat/123",
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
	runCalls          []string
	runErrors         map[string]error
	runErrorSequences map[string][]error
}

func (r *fakeRunner) Run(ctx context.Context, name string, args ...string) error {
	_ = ctx
	call := name + " " + joinArgs(args)
	r.runCalls = append(r.runCalls, call)
	if sequence := r.runErrorSequences[call]; len(sequence) > 0 {
		err := sequence[0]
		r.runErrorSequences[call] = sequence[1:]
		return err
	}
	if r.runErrors != nil && r.runErrors[call] != nil {
		return r.runErrors[call]
	}
	return nil
}

type exitCodeError struct {
	code int
}

func (e exitCodeError) Error() string {
	return "exit status"
}

func (e exitCodeError) ExitCode() int {
	return e.code
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
