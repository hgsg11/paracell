package review

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hgsg11/paracell/internal/adapter/system"
	"github.com/hgsg11/paracell/internal/domain"
	"github.com/hgsg11/paracell/internal/usecase"
)

// fakeRunner records every command with the directory it ran from and returns
// scripted outputs by command line.
type fakeRunner struct {
	dir     string
	calls   *[]string
	outputs map[string]string
	errors  map[string]error
}

func newFakeRunner() fakeRunner {
	return fakeRunner{calls: &[]string{}, outputs: map[string]string{}, errors: map[string]error{}}
}

func (f fakeRunner) InDir(dir string) system.Runner {
	f.dir = dir
	return f
}

func (f fakeRunner) Run(ctx context.Context, name string, args ...string) error {
	_, err := f.Output(ctx, name, args...)
	return err
}

func (f fakeRunner) Output(_ context.Context, name string, args ...string) (string, error) {
	call := strings.Join(append([]string{name}, args...), " ")
	if f.dir != "" {
		call = "[" + f.dir + "] " + call
	}
	*f.calls = append(*f.calls, call)
	return f.outputs[call], f.errors[call]
}

var source = domain.NewSourceResource("api", ".paracell/cells/175/api", "origin/main", "feat/175")

const worktree = "/project/.paracell/cells/175/api"

func TestInspectSourceは未Commitの変更とBaseにないCommitを調べる(t *testing.T) {
	runner := newFakeRunner()
	runner.outputs["git -C "+worktree+" status --porcelain"] = " M main.go"
	runner.outputs["git -C "+worktree+" rev-list --count origin/main..HEAD"] = "2"
	changes, err := NewGitHubAdapter(runner, "/project").InspectSource(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	if changes != (usecase.SourceChanges{Uncommitted: true, Committed: true}) {
		t.Fatalf("changes = %#v", changes)
	}
	runner.outputs["git -C "+worktree+" status --porcelain"] = ""
	runner.outputs["git -C "+worktree+" rev-list --count origin/main..HEAD"] = "0"
	changes, err = NewGitHubAdapter(runner, "/project").InspectSource(context.Background(), source)
	if err != nil || changes != (usecase.SourceChanges{}) {
		t.Fatalf("changes = %#v, err = %v", changes, err)
	}
}

func TestPushはCellのBranchをOriginへ送る(t *testing.T) {
	runner := newFakeRunner()
	if err := NewGitHubAdapter(runner, "/project").Push(context.Background(), source); err != nil {
		t.Fatal(err)
	}
	if want := []string{"git -C " + worktree + " push --force-with-lease -u origin feat/175"}; !reflect.DeepEqual(*runner.calls, want) {
		t.Fatalf("calls = %q", *runner.calls)
	}
}

func TestOpenPullRequestはBaseと閉じる記法を付けてPRを作り番号を返す(t *testing.T) {
	runner := newFakeRunner()
	create := "[" + worktree + "] gh pr create --head feat/175 --title feat: x --body body\n\nCloses #175\n --base main"
	runner.outputs[create] = "Creating pull request\nhttps://github.com/hgsg11/paracell/pull/180\n"
	number, err := NewGitHubAdapter(runner, "/project").OpenPullRequest(context.Background(), source, "175", "feat: x", "body\n")
	if err != nil {
		t.Fatal(err)
	}
	if number != "180" {
		t.Fatalf("number = %q", number)
	}
	want := []string{"[" + worktree + "] gh pr list --head feat/175 --state open --json number --jq .[0].number", create}
	if !reflect.DeepEqual(*runner.calls, want) {
		t.Fatalf("calls = %q", *runner.calls)
	}
}

func TestOpenPullRequestは開いているPRがあれば作らずにその番号を返す(t *testing.T) {
	runner := newFakeRunner()
	runner.outputs["["+worktree+"] gh pr list --head feat/175 --state open --json number --jq .[0].number"] = "77\n"
	number, err := NewGitHubAdapter(runner, "/project").OpenPullRequest(context.Background(), source, "175", "t", "b")
	if err != nil || number != "77" || len(*runner.calls) != 1 {
		t.Fatalf("number = %q, err = %v, calls = %q", number, err, *runner.calls)
	}
}

func TestWithClosingReferenceは本文の末尾に一度だけ付ける(t *testing.T) {
	for _, tc := range []struct{ body, issue, want string }{
		{"body", "175", "body\n\nCloses #175\n"},
		{"body", "#175", "body\n\nCloses #175\n"},
		{"", "175", "Closes #175\n"},
		{"fix\n\nCloses #175\n", "175", "fix\n\nCloses #175\n"},
		{"body", "PROJ-1", "body"},
	} {
		if got := withClosingReference(tc.body, tc.issue); got != tc.want {
			t.Fatalf("withClosingReference(%q, %q) = %q, want %q", tc.body, tc.issue, got, tc.want)
		}
	}
	for base, want := range map[string]string{"origin/main": "main", "main": "main", "refs/remotes/origin/dev": "dev", "": "", "current": ""} {
		if got := baseBranch(base); got != want {
			t.Fatalf("baseBranch(%q) = %q, want %q", base, got, want)
		}
	}
}

func TestPullRequestは状態と著者ごとの最新レビューとイベントを読む(t *testing.T) {
	runner := newFakeRunner()
	runner.outputs["["+worktree+"] gh pr view 180 --json state,reviews,comments"] = `{
  "state": "OPEN",
  "reviews": [
    {"author": {"login": "alice"}, "state": "CHANGES_REQUESTED", "submittedAt": "2026-11-23T01:00:00Z"},
    {"author": {"login": "bob"}, "state": "COMMENTED", "submittedAt": "2026-11-23T02:00:00Z"},
    {"author": {"login": "alice"}, "state": "APPROVED", "submittedAt": "2026-11-23T03:00:00Z"},
    {"author": {"login": "carol"}, "state": "PENDING", "submittedAt": null}
  ],
  "comments": [{"author": {"login": "bob"}, "createdAt": "2026-11-23T04:00:00Z"}]
}`
	report, err := NewGitHubAdapter(runner, "/project").PullRequest(context.Background(), source, "180")
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != (domain.PullRequestStatus{State: domain.PullRequestOpen, Approval: domain.ApprovalApproved}) {
		t.Fatalf("status = %#v", report.Status)
	}
	at := func(hour int) time.Time { return time.Date(2026, 11, 23, hour, 0, 0, 0, time.UTC) }
	want := []usecase.ReviewEvent{
		{Kind: usecase.ReviewEventReview, Verdict: usecase.VerdictChangesRequested, At: at(1)},
		{Kind: usecase.ReviewEventReview, Verdict: usecase.VerdictCommented, At: at(2)},
		{Kind: usecase.ReviewEventReview, Verdict: usecase.VerdictApproved, At: at(3)},
		{Kind: usecase.ReviewEventComment, At: at(4)},
	}
	if len(report.Events) != len(want) {
		t.Fatalf("events = %#v", report.Events)
	}
	for i := range want {
		if report.Events[i].Kind != want[i].Kind || report.Events[i].Verdict != want[i].Verdict || !report.Events[i].At.Equal(want[i].At) {
			t.Fatalf("event %d = %#v, want %#v", i, report.Events[i], want[i])
		}
	}
}

func TestPullRequestは残っているRequestChangesを優先する(t *testing.T) {
	for state, want := range map[string]domain.PullRequestState{"MERGED": domain.PullRequestMerged, "CLOSED": domain.PullRequestClosed} {
		report, err := parsePullRequest(`{"state": "` + state + `", "reviews": [
  {"author": {"login": "alice"}, "state": "APPROVED", "submittedAt": "2026-11-23T01:00:00Z"},
  {"author": {"login": "bob"}, "state": "CHANGES_REQUESTED", "submittedAt": "2026-11-23T02:00:00Z"}
], "comments": []}`)
		if err != nil {
			t.Fatal(err)
		}
		if report.Status.State != want || report.Status.Approval != domain.ApprovalChangesRequested {
			t.Fatalf("%s: status = %#v", state, report.Status)
		}
	}
	report, err := parsePullRequest(`{"state": "OPEN", "reviews": [
  {"author": {"login": "bob"}, "state": "CHANGES_REQUESTED", "submittedAt": "2026-11-23T01:00:00Z"},
  {"author": {"login": "bob"}, "state": "DISMISSED", "submittedAt": "2026-11-23T02:00:00Z"}
], "comments": []}`)
	if err != nil || report.Status.Approval != domain.ApprovalNone {
		t.Fatalf("dismissed: status = %#v, err = %v", report.Status, err)
	}
	if _, err := parsePullRequest(`{"state": "DRAFT"}`); err == nil {
		t.Fatal("unknown state returned no error")
	}
}

func TestMergePullRequestは方法を指定してmergeし失敗を返す(t *testing.T) {
	runner := newFakeRunner()
	runner.errors["["+worktree+"] gh pr merge 180 --rebase"] = errors.New("required status check is failing")
	adapter := NewGitHubAdapter(runner, "/project")
	if err := adapter.MergePullRequest(context.Background(), source, "180", usecase.MergeCommit); err != nil {
		t.Fatal(err)
	}
	if err := adapter.MergePullRequest(context.Background(), source, "180", usecase.MergeRebase); err == nil {
		t.Fatal("no error")
	}
	if err := adapter.MergePullRequest(context.Background(), source, "180", "fast-forward"); err == nil {
		t.Fatal("unknown method returned no error")
	}
	if want := []string{"[" + worktree + "] gh pr merge 180 --merge", "[" + worktree + "] gh pr merge 180 --rebase"}; !reflect.DeepEqual(*runner.calls, want) {
		t.Fatalf("calls = %q", *runner.calls)
	}
}
