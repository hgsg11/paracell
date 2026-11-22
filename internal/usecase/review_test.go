package usecase

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hgsg11/paracell/internal/domain"
)

// fakeReview is a review provider whose pull requests are scripted per poll.
type fakeReview struct {
	changes  map[string]SourceChanges
	reports  map[string][]PullRequestReport
	polls    map[string]int
	next     int
	calls    []string
	mergeErr error
}

func newFakeReview() *fakeReview {
	return &fakeReview{changes: map[string]SourceChanges{}, reports: map[string][]PullRequestReport{}, polls: map[string]int{}, next: 180}
}

func (f *fakeReview) Review(driver domain.ReviewDriverType) (ReviewPort, error) {
	if driver != domain.GitHubReview {
		return nil, errors.New("unexpected driver " + string(driver))
	}
	return f, nil
}

func (f *fakeReview) InspectSource(_ context.Context, source domain.SourceResource) (SourceChanges, error) {
	return f.changes[source.WorktreePath], nil
}

func (f *fakeReview) Push(_ context.Context, source domain.SourceResource) error {
	f.calls = append(f.calls, "push "+source.WorktreePath)
	return nil
}

func (f *fakeReview) OpenPullRequest(_ context.Context, source domain.SourceResource, issue string, title string, body string) (string, error) {
	number := strconv.Itoa(f.next)
	f.next++
	f.calls = append(f.calls, "open "+source.WorktreePath+" "+issue+" "+title+" "+body)
	return number, nil
}

// PullRequest returns the scripted report of the current poll, and keeps
// returning the last one once the script runs out.
func (f *fakeReview) PullRequest(_ context.Context, _ domain.SourceResource, number string) (PullRequestReport, error) {
	script := f.reports[number]
	if len(script) == 0 {
		return PullRequestReport{Status: domain.PullRequestStatus{State: domain.PullRequestOpen, Approval: domain.ApprovalNone}}, nil
	}
	i := min(f.polls[number], len(script)-1)
	f.polls[number]++
	return script[i], nil
}

func (f *fakeReview) MergePullRequest(_ context.Context, _ domain.SourceResource, number string, method MergeMethod) error {
	f.calls = append(f.calls, "merge "+number+" "+string(method))
	return f.mergeErr
}

func openReport(approval domain.PullRequestApproval, events ...ReviewEvent) PullRequestReport {
	return PullRequestReport{Status: domain.PullRequestStatus{State: domain.PullRequestOpen, Approval: approval}, Events: events}
}

func stateReport(state domain.PullRequestState) PullRequestReport {
	return PullRequestReport{Status: domain.PullRequestStatus{State: state, Approval: domain.ApprovalNone}}
}

// newReviewPorts returns ports with one Cell "175" that has two targets with
// their own worktrees, api and web, and a review provider.
func newReviewPorts(t *testing.T) (*fakePorts, *fakeReview) {
	t.Helper()
	ports := newFakePorts()
	ports.config.ReviewDriverType = domain.GitHubReview
	ports.cells = newUsecaseCellSet(t, [3]string{"cell-1", "175", "feat"})
	for _, target := range []struct{ id, name, path string }{{"t-api", "api", "api"}, {"t-web", "web", "web"}} {
		source, err := domain.NewSource(target.path, "origin/main", "feat/175")
		if err != nil {
			t.Fatal(err)
		}
		cell, err := domain.NewTargetCell(target.id, "group-cell-1", target.name, &source, nil)
		if err != nil {
			t.Fatal(err)
		}
		ports.cells.Targets = append(ports.cells.Targets, cell)
	}
	return ports, newFakeReview()
}

func newRequestReview(ports *fakePorts, review *fakeReview) (RequestReviewUseCase, *int) {
	sleeps := 0
	return RequestReviewUseCase{
		Config: ports, Cells: ports, ReviewFactory: review, IDs: &sequenceIDGenerator{}, Fork: newForkCellUseCase(ports),
		Sleep: func(context.Context, time.Duration) error {
			sleeps++
			if sleeps > 20 {
				return errors.New("waited too long")
			}
			return nil
		},
	}, &sleeps
}

func ptr(value string) *string { return &value }

const apiWorktree = ".paracell/cells/175/api"
const webWorktree = ".paracell/cells/175/web"

func TestRequestReviewは変更のあるTargetごとにPRを開きReviewを保存してレビュー中にする(t *testing.T) {
	ports, review := newReviewPorts(t)
	review.changes[apiWorktree] = SourceChanges{Committed: true}
	at := time.Date(2026, 11, 23, 10, 0, 0, 0, time.UTC)
	review.reports["180"] = []PullRequestReport{openReport(domain.ApprovalNone), openReport(domain.ApprovalChangesRequested, ReviewEvent{Kind: ReviewEventReview, Verdict: VerdictChangesRequested, At: at})}
	uc, _ := newRequestReview(ports, review)

	message, err := uc.Execute(context.Background(), RequestReviewInput{Cell: "175", Title: ptr("feat: x"), Body: ptr("body")})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"push " + apiWorktree, "open " + apiWorktree + " 175 feat: x body"}; strings.Join(review.calls, "\n") != strings.Join(want, "\n") {
		t.Fatalf("calls = %q", review.calls)
	}
	if len(ports.cells.Reviews) != 1 {
		t.Fatalf("reviews = %#v", ports.cells.Reviews)
	}
	saved := ports.cells.Reviews[0]
	if saved.CellGroupID != "group-cell-1" || len(saved.PullRequests) != 1 || saved.PullRequests[0] != (domain.PullRequestRef{TargetCellID: "t-api", Number: "180"}) {
		t.Fatalf("review = %#v", saved)
	}
	if !saved.HandledUntil.Equal(at) {
		t.Fatalf("handled until = %s, want %s", saved.HandledUntil, at)
	}
	if ports.cells.Commanders[0].Progress != domain.InReview {
		t.Fatalf("progress = %s", ports.cells.Commanders[0].Progress)
	}
	for _, want := range []string{"PR #180", "新しいレビュー 1 件（Request changes 1）", "paracell implement", "paracell review"} {
		if !strings.Contains(message, want) {
			t.Fatalf("message %q does not contain %q", message, want)
		}
	}
}

func TestRequestReviewは同じイベントを二度届けず直している間に来たものはすぐ返す(t *testing.T) {
	ports, review := newReviewPorts(t)
	review.changes[apiWorktree] = SourceChanges{Committed: true}
	first := time.Date(2026, 11, 23, 10, 0, 0, 0, time.UTC)
	second := first.Add(time.Hour)
	comment := func(at time.Time) ReviewEvent { return ReviewEvent{Kind: ReviewEventComment, At: at} }
	review.reports["180"] = []PullRequestReport{
		openReport(domain.ApprovalNone, comment(first)),
		// While the agent was fixing, another comment arrived.
		openReport(domain.ApprovalNone, comment(first), comment(second)),
	}
	uc, sleeps := newRequestReview(ports, review)
	if _, err := uc.Execute(context.Background(), RequestReviewInput{Cell: "175", Title: ptr("t"), Body: ptr("b")}); err != nil {
		t.Fatal(err)
	}
	ports.cells.Commanders[0].Progress = domain.InProgress
	message, err := uc.Execute(context.Background(), RequestReviewInput{Cell: "175"})
	if err != nil {
		t.Fatal(err)
	}
	if *sleeps != 0 {
		t.Fatalf("the second review waited %d times instead of returning at once", *sleeps)
	}
	if !strings.Contains(message, "新しいコメント 1 件") || !ports.cells.Reviews[0].HandledUntil.Equal(second) {
		t.Fatalf("message = %q, review = %#v", message, ports.cells.Reviews[0])
	}
	if ports.cells.Commanders[0].Progress != domain.InReview {
		t.Fatalf("progress = %s", ports.cells.Commanders[0].Progress)
	}
	// Nothing new: the third review waits until the PR is merged on GitHub.
	review.reports["180"] = append(review.reports["180"], openReport(domain.ApprovalNone, comment(first), comment(second)), stateReport(domain.PullRequestMerged))
	message, err = uc.Execute(context.Background(), RequestReviewInput{Cell: "175"})
	if err != nil {
		t.Fatal(err)
	}
	if *sleeps != 1 || !strings.Contains(message, "merge された") {
		t.Fatalf("sleeps = %d, message = %q", *sleeps, message)
	}
	if ports.cells.Commanders[0].Progress != domain.Merged || len(ports.cells.Reviews) != 0 {
		t.Fatalf("progress = %s, reviews = %#v", ports.cells.Commanders[0].Progress, ports.cells.Reviews)
	}
	for _, call := range review.calls {
		if strings.HasPrefix(call, "merge") {
			t.Fatalf("a PR merged on GitHub was merged again: %q", review.calls)
		}
	}
}

func TestRequestReviewの引数と未CommitとTargetの検証(t *testing.T) {
	ports, review := newReviewPorts(t)
	uc, _ := newRequestReview(ports, review)
	if _, err := uc.Execute(context.Background(), RequestReviewInput{Cell: "175"}); err == nil || !strings.Contains(err.Error(), "--title") {
		t.Fatalf("first review without title: %v", err)
	}
	if _, err := uc.Execute(context.Background(), RequestReviewInput{Cell: "175", Title: ptr("t"), Body: ptr("b")}); err == nil || !strings.Contains(err.Error(), "no target has commits") {
		t.Fatalf("no changes: %v", err)
	}
	review.changes[webWorktree] = SourceChanges{Committed: true, Uncommitted: true}
	if _, err := uc.Execute(context.Background(), RequestReviewInput{Cell: "175", Title: ptr("t"), Body: ptr("b")}); err == nil || !strings.Contains(err.Error(), "uncommitted") {
		t.Fatalf("uncommitted: %v", err)
	}
	if len(review.calls) != 0 || len(ports.cells.Reviews) != 0 || ports.cells.Commanders[0].Progress != domain.InProgress {
		t.Fatalf("calls = %q, state = %#v", review.calls, ports.cells)
	}
	review.changes[webWorktree] = SourceChanges{Committed: true}
	review.reports["180"] = []PullRequestReport{openReport(domain.ApprovalNone, ReviewEvent{Kind: ReviewEventComment, At: time.Now()})}
	if _, err := uc.Execute(context.Background(), RequestReviewInput{Cell: "175", Title: ptr("t"), Body: ptr("b")}); err != nil {
		t.Fatal(err)
	}
	if _, err := uc.Execute(context.Background(), RequestReviewInput{Cell: "175", Title: ptr("t"), Body: ptr("b")}); err == nil || !strings.Contains(err.Error(), "without --title") {
		t.Fatalf("second review with title: %v", err)
	}
	review.changes[webWorktree] = SourceChanges{Uncommitted: true}
	if _, err := uc.Execute(context.Background(), RequestReviewInput{Cell: "175"}); err == nil || !strings.Contains(err.Error(), "uncommitted") {
		t.Fatalf("second review with uncommitted changes: %v", err)
	}
}

func TestRequestReviewは全部Approveされたらmergeしキューを進める(t *testing.T) {
	ports, review := newReviewPorts(t)
	ports.cells.Groups[0].QueueEntryID = "entry-1"
	review.changes[apiWorktree] = SourceChanges{Committed: true}
	review.changes[webWorktree] = SourceChanges{Committed: true}
	review.reports["180"] = []PullRequestReport{openReport(domain.ApprovalApproved)}
	review.reports["181"] = []PullRequestReport{openReport(domain.ApprovalNone), openReport(domain.ApprovalApproved)}
	uc, sleeps := newRequestReview(ports, review)

	message, err := uc.Execute(context.Background(), RequestReviewInput{Cell: "175", Title: ptr("t"), Body: ptr("b")})
	if err != nil {
		t.Fatal(err)
	}
	if *sleeps != 1 || !strings.Contains(message, "Approve されたので merge した") {
		t.Fatalf("sleeps = %d, message = %q", *sleeps, message)
	}
	if got := review.calls[len(review.calls)-2:]; got[0] != "merge 180 merge" || got[1] != "merge 181 merge" {
		t.Fatalf("calls = %q", review.calls)
	}
	if ports.cells.Commanders[0].Progress != domain.Merged || len(ports.cells.Reviews) != 0 {
		t.Fatalf("state = %#v", ports.cells)
	}
}

func TestRequestReviewはmergeに失敗したら理由を出して状態を変えない(t *testing.T) {
	ports, review := newReviewPorts(t)
	review.changes[apiWorktree] = SourceChanges{Committed: true}
	review.reports["180"] = []PullRequestReport{openReport(domain.ApprovalApproved)}
	review.mergeErr = errors.New("required status check is failing")
	uc, _ := newRequestReview(ports, review)

	_, err := uc.Execute(context.Background(), RequestReviewInput{Cell: "175", Title: ptr("t"), Body: ptr("b")})
	if err == nil || !strings.Contains(err.Error(), "required status check is failing") {
		t.Fatalf("error = %v", err)
	}
	if ports.cells.Commanders[0].Progress != domain.InReview || len(ports.cells.Reviews) != 1 {
		t.Fatalf("state changed: %#v", ports.cells)
	}
}

func TestRequestReviewはcloseされたらabandonedにし混在なら決めずに返す(t *testing.T) {
	ports, review := newReviewPorts(t)
	review.changes[apiWorktree] = SourceChanges{Committed: true}
	review.changes[webWorktree] = SourceChanges{Committed: true}
	review.reports["180"] = []PullRequestReport{stateReport(domain.PullRequestMerged)}
	review.reports["181"] = []PullRequestReport{stateReport(domain.PullRequestClosed)}
	uc, _ := newRequestReview(ports, review)
	message, err := uc.Execute(context.Background(), RequestReviewInput{Cell: "175", Title: ptr("t"), Body: ptr("b")})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(message, "paracell merged 175") || !strings.Contains(message, "paracell abandon 175") {
		t.Fatalf("message = %q", message)
	}
	if ports.cells.Commanders[0].Progress != domain.InReview || len(ports.cells.Reviews) != 1 {
		t.Fatalf("mixed outcome changed the state: %#v", ports.cells)
	}
	outcomes, err := (CheckReviewsUseCase{Config: ports, Cells: ports, ReviewFactory: review}).Execute(context.Background())
	if err != nil || outcomes["175"] != domain.ReviewMixed {
		t.Fatalf("outcomes = %v, err = %v", outcomes, err)
	}
	// A person decides: merged removes the Review too.
	if _, err := (MarkCellMergedUseCase{Cells: ports, Fork: newForkCellUseCase(ports)}).Execute(context.Background(), MarkCellMergedInput{Cell: "175"}); err != nil {
		t.Fatal(err)
	}
	if len(ports.cells.Reviews) != 0 {
		t.Fatalf("reviews = %#v", ports.cells.Reviews)
	}

	ports, review = newReviewPorts(t)
	review.changes[apiWorktree] = SourceChanges{Committed: true}
	review.reports["180"] = []PullRequestReport{stateReport(domain.PullRequestClosed)}
	uc, _ = newRequestReview(ports, review)
	if _, err := uc.Execute(context.Background(), RequestReviewInput{Cell: "175", Title: ptr("t"), Body: ptr("b")}); err != nil {
		t.Fatal(err)
	}
	if ports.cells.Commanders[0].Progress != domain.Abandoned || len(ports.cells.Reviews) != 0 {
		t.Fatalf("state = %#v", ports.cells)
	}
}

func TestRequestReviewはReview設定がなければレビュー中にするだけ(t *testing.T) {
	ports, review := newReviewPorts(t)
	ports.config.ReviewDriverType = domain.NoReview
	uc, _ := newRequestReview(ports, review)
	message, err := uc.Execute(context.Background(), RequestReviewInput{Cell: "175"})
	if err != nil || message != "" {
		t.Fatalf("message = %q, err = %v", message, err)
	}
	if ports.cells.Commanders[0].Progress != domain.InReview || len(review.calls) != 0 || len(ports.cells.Reviews) != 0 {
		t.Fatalf("state = %#v, calls = %q", ports.cells, review.calls)
	}
}

func TestMergeCellはmerge済みのPRをmergeせず残りをmergeしてmergedにする(t *testing.T) {
	ports, review := newReviewPorts(t)
	ports.cells.Groups[0].QueueEntryID = "entry-1"
	ports.cells.Commanders[0].Progress = domain.InReview
	ports.cells.Reviews = []domain.Review{mustReview(t, "group-cell-1", "180", "181")}
	review.reports["180"] = []PullRequestReport{stateReport(domain.PullRequestMerged)}
	review.reports["181"] = []PullRequestReport{openReport(domain.ApprovalNone)}
	uc := MergeCellUseCase{Config: ports, Cells: ports, ReviewFactory: review, Fork: newForkCellUseCase(ports)}

	cell, err := uc.Execute(context.Background(), MergeCellInput{Cell: "175", Method: MergeSquash})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(review.calls, ",") != "merge 181 squash" {
		t.Fatalf("calls = %q", review.calls)
	}
	if cell.Progress != domain.Merged || len(ports.cells.Reviews) != 0 {
		t.Fatalf("cell = %#v, reviews = %#v", cell, ports.cells.Reviews)
	}
	if _, err := uc.Execute(context.Background(), MergeCellInput{Cell: "175"}); !errors.Is(err, ErrNoReview) {
		t.Fatalf("merge without review: %v", err)
	}
}

func TestMergeCellは失敗したら状態を変えない(t *testing.T) {
	ports, review := newReviewPorts(t)
	ports.cells.Commanders[0].Progress = domain.InReview
	ports.cells.Reviews = []domain.Review{mustReview(t, "group-cell-1", "180")}
	review.mergeErr = errors.New("not mergeable")
	uc := MergeCellUseCase{Config: ports, Cells: ports, ReviewFactory: review, Fork: newForkCellUseCase(ports)}
	if _, err := uc.Execute(context.Background(), MergeCellInput{Cell: "175"}); err == nil {
		t.Fatal("no error")
	}
	if ports.cells.Commanders[0].Progress != domain.InReview || len(ports.cells.Reviews) != 1 || ports.saves != 0 {
		t.Fatalf("state changed: %#v", ports.cells)
	}
	review.mergeErr = nil
	review.reports["180"] = []PullRequestReport{stateReport(domain.PullRequestClosed)}
	if _, err := uc.Execute(context.Background(), MergeCellInput{Cell: "175"}); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Fatalf("closed PR: %v", err)
	}
	if len(review.calls) != 1 {
		t.Fatalf("calls = %q", review.calls)
	}
}

func mustReview(t *testing.T, groupID string, numbers ...string) domain.Review {
	t.Helper()
	targets := []string{"t-api", "t-web"}
	var refs []domain.PullRequestRef
	for i, number := range numbers {
		refs = append(refs, domain.PullRequestRef{TargetCellID: targets[i], Number: number})
	}
	review, err := domain.NewReview("review-1", groupID, refs)
	if err != nil {
		t.Fatal(err)
	}
	return review
}
