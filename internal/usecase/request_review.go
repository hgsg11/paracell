package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hgsg11/paracell/internal/domain"
)

// DefaultReviewPollInterval is how often a waiting review asks the provider.
const DefaultReviewPollInterval = 45 * time.Second

type RequestReviewInput struct {
	Cell string
	// Title and Body are required by the first request, which opens the pull
	// requests, and rejected by every later one.
	Title *string
	Body  *string
}

// RequestReviewUseCase hands the committed work of a Cell to review and waits
// until the agent has something to do or the review ends. Without a review
// provider it only marks the Cell in review.
type RequestReviewUseCase struct {
	Config        ConfigPort
	Cells         CellPort
	ReviewFactory ReviewProviderFactory
	IDs           IDGenerator
	// Fork starts the queued Cells released when the review ends.
	Fork ForkCellUseCase
	// Interval is the time between two polls; zero means DefaultReviewPollInterval.
	Interval time.Duration
	// Sleep waits between two polls; nil waits on a timer.
	Sleep func(ctx context.Context, d time.Duration) error
	// Report receives progress lines while the review starts; nil drops them.
	Report func(line string)
}

// Execute returns the short text that tells the agent what to do next.
func (u RequestReviewUseCase) Execute(ctx context.Context, input RequestReviewInput) (string, error) {
	templates, err := u.Config.Load(ctx)
	if err != nil {
		return "", err
	}
	if !templates.ReviewDriverType.Enabled() {
		_, err := (MarkCellInReviewUseCase{Cells: u.Cells}).Execute(ctx, MarkCellInReviewInput{Cell: input.Cell})
		return "", err
	}
	port, err := u.ReviewFactory.Review(templates.ReviewDriverType)
	if err != nil {
		return "", err
	}
	set, err := u.Cells.LoadCells(ctx)
	if err != nil {
		return "", err
	}
	commander, group, err := set.resolve(input.Cell)
	if err != nil {
		return "", err
	}
	if commander.Progress.IsTerminal() {
		return "", fmt.Errorf("cell %s is already %s", group.Name().Value, commander.Progress)
	}
	sources := reviewSources(group, set.Targets)
	changes, err := inspectSources(ctx, port, sources)
	if err != nil {
		return "", err
	}
	if review, ok := set.review(group.ID); ok {
		if input.Title != nil || input.Body != nil {
			return "", errors.New("the pull requests are already open: run paracell review without --title and --body-file")
		}
		if err := u.pushReview(ctx, port, review, sources); err != nil {
			return "", err
		}
	} else {
		if input.Title == nil || input.Body == nil {
			return "", errors.New("the first paracell review requires --title and --body-file")
		}
		if err := u.openReview(ctx, port, group, sources, changes, *input.Title, *input.Body); err != nil {
			return "", err
		}
	}
	if commander.Progress == domain.InProgress {
		if _, _, err := updateCommander(ctx, u.Cells, group.ID, (*domain.CommanderCell).MarkInReview); err != nil {
			return "", err
		}
	}
	return u.wait(ctx, port, group.ID)
}

// inspectSources fails when any worktree has uncommitted changes, and returns
// what each worktree holds otherwise.
func inspectSources(ctx context.Context, port ReviewPort, sources []reviewSource) ([]SourceChanges, error) {
	changes := make([]SourceChanges, 0, len(sources))
	var dirty []string
	for _, source := range sources {
		change, err := port.InspectSource(ctx, source.Resource)
		if err != nil {
			return nil, fmt.Errorf("inspect %s: %w", source.Resource.WorktreePath, err)
		}
		if change.Uncommitted {
			dirty = append(dirty, source.Resource.WorktreePath)
		}
		changes = append(changes, change)
	}
	if len(dirty) > 0 {
		return nil, fmt.Errorf("commit the changes first; uncommitted changes in %s", strings.Join(dirty, ", "))
	}
	return changes, nil
}

// openReview pushes every worktree with commits, opens a pull request for each,
// and saves the Review. A pull request that is already open is reused, so a
// rerun after a failed save does not open a second one.
func (u RequestReviewUseCase) openReview(ctx context.Context, port ReviewPort, group domain.CellGroup, sources []reviewSource, changes []SourceChanges, title string, body string) error {
	var refs []domain.PullRequestRef
	for i, source := range sources {
		if !changes[i].Committed {
			continue
		}
		if err := port.Push(ctx, source.Resource); err != nil {
			return fmt.Errorf("push %s: %w", source.Resource.WorktreePath, err)
		}
		number, err := port.OpenPullRequest(ctx, source.Resource, group.Issue, title, body)
		if err != nil {
			return fmt.Errorf("open pull request for %s: %w", source.Resource.WorktreePath, err)
		}
		u.report(fmt.Sprintf("PR #%s を開いた (%s)", number, source.TargetName))
		ref, err := domain.NewPullRequestRef(source.TargetCellID, number)
		if err != nil {
			return err
		}
		refs = append(refs, ref)
	}
	if len(refs) == 0 {
		return errors.New("no target has commits to review")
	}
	review, err := domain.NewReview(u.IDs.NewID(), group.ID, refs)
	if err != nil {
		return err
	}
	return u.Cells.UpdateCells(ctx, func(set CellSet) (CellSet, error) {
		if _, exists := set.review(group.ID); exists {
			return CellSet{}, fmt.Errorf("cell %s already has a review", group.Name().Value)
		}
		set.Reviews = append(set.Reviews, review)
		return set, nil
	})
}

// pushReview pushes the worktree of every pull request of the Review.
func (u RequestReviewUseCase) pushReview(ctx context.Context, port ReviewPort, review domain.Review, sources []reviewSource) error {
	pullRequests, err := pullRequestSources(review, sources)
	if err != nil {
		return err
	}
	for _, pullRequest := range pullRequests {
		if err := port.Push(ctx, pullRequest.Resource); err != nil {
			return fmt.Errorf("push %s: %w", pullRequest.Resource.WorktreePath, err)
		}
	}
	u.report(fmt.Sprintf("PR %s に push した", formatPullRequestNumbers(pullRequests)))
	return nil
}

// wait polls the provider until the review ends or new reviews or comments
// arrive. It merges the pull requests once every open one is approved.
func (u RequestReviewUseCase) wait(ctx context.Context, port ReviewPort, groupID string) (string, error) {
	u.report("レビューを待っている")
	for {
		message, done, err := u.poll(ctx, port, groupID)
		if err != nil || done {
			return message, err
		}
		if err := u.sleep(ctx); err != nil {
			return "", err
		}
	}
}

func (u RequestReviewUseCase) poll(ctx context.Context, port ReviewPort, groupID string) (string, bool, error) {
	set, err := u.Cells.LoadCells(ctx)
	if err != nil {
		return "", true, err
	}
	group, ok := set.group(groupID)
	if !ok {
		return "", true, fmt.Errorf("CellGroup %q: %w", groupID, domain.ErrNotFound)
	}
	review, ok := set.review(groupID)
	if !ok {
		return "レビューは終わった（Cell は merged か abandoned になった）。これ以上することはない", true, nil
	}
	pullRequests, err := pullRequestSources(review, reviewSources(group, set.Targets))
	if err != nil {
		return "", true, err
	}
	reports, err := fetchPullRequests(ctx, port, pullRequests)
	if err != nil {
		return "", true, err
	}
	statuses := pullRequestStatuses(reports)
	name := group.Name().Value
	switch domain.DecideReviewOutcome(statuses) {
	case domain.ReviewMerged:
		return u.finish(ctx, name, (*domain.CommanderCell).MarkMerged, fmt.Sprintf("PR %s はすべて merge された。Cell を merged にした。これ以上することはない", formatPullRequestNumbers(pullRequests)))
	case domain.ReviewClosed:
		return u.finish(ctx, name, (*domain.CommanderCell).MarkAbandoned, fmt.Sprintf("PR %s はすべて merge されずに close された。Cell を abandoned にした。これ以上することはない", formatPullRequestNumbers(pullRequests)))
	case domain.ReviewMixed:
		return fmt.Sprintf("PR %s の一部が merge され、残りが close された。自動では決めない。人が paracell merged %s か paracell abandon %s を選ぶのを待つ", formatPullRequestNumbers(pullRequests), name, name), true, nil
	}
	if domain.IsReviewApproved(statuses) {
		if err := mergeOpenPullRequests(ctx, port, pullRequests, reports, MergeCommit); err != nil {
			return "", true, fmt.Errorf("PR %s はすべて Approve されたが merge に失敗した。原因を直して commit し、paracell review を実行する: %w", formatPullRequestNumbers(pullRequests), err)
		}
		return u.finish(ctx, name, (*domain.CommanderCell).MarkMerged, fmt.Sprintf("PR %s はすべて Approve されたので merge した。Cell を merged にした。これ以上することはない", formatPullRequestNumbers(pullRequests)))
	}
	summary, latest := summarizeEvents(review, pullRequests, reports)
	if summary == "" {
		return "", false, nil
	}
	if err := u.acknowledge(ctx, review.ID, latest); err != nil {
		return "", true, err
	}
	return summary + "。gh で読んで対応し、直したら paracell implement → 修正 → commit → paracell review。PR にはコメントを書かない（対応内容は commit メッセージに書く）", true, nil
}

// finish moves the Cell to merged or abandoned with its Review and QueueEntry
// removed, and starts the queued Cells it released.
func (u RequestReviewUseCase) finish(ctx context.Context, cell string, change func(*domain.CommanderCell) error, message string) (string, bool, error) {
	if _, err := finishCommander(ctx, u.Cells, u.Fork, cell, change); err != nil {
		return "", true, err
	}
	return message, true, nil
}

// acknowledge advances HandledUntil of the Review to until.
func (u RequestReviewUseCase) acknowledge(ctx context.Context, reviewID string, until time.Time) error {
	return u.Cells.UpdateCells(ctx, func(set CellSet) (CellSet, error) {
		for _, review := range set.Reviews {
			if review.ID != reviewID {
				continue
			}
			if err := review.Acknowledge(until); err != nil {
				return CellSet{}, err
			}
			if err := set.replaceReview(review); err != nil {
				return CellSet{}, err
			}
			return set, nil
		}
		return CellSet{}, fmt.Errorf("review %q: %w", reviewID, domain.ErrNotFound)
	})
}

// summarizeEvents describes the reviews and comments written after the
// Review's HandledUntil, per pull request, and returns the time of the latest
// one. The summary is empty when nothing new arrived.
func summarizeEvents(review domain.Review, pullRequests []pullRequestSource, reports []PullRequestReport) (string, time.Time) {
	var parts []string
	latest := review.HandledUntil
	for i, report := range reports {
		reviews, comments := 0, 0
		verdicts := map[ReviewVerdict]int{}
		for _, event := range report.Events {
			if !event.At.After(review.HandledUntil) {
				continue
			}
			if event.At.After(latest) {
				latest = event.At
			}
			if event.Kind == ReviewEventReview {
				reviews++
				verdicts[event.Verdict]++
			} else {
				comments++
			}
		}
		if reviews+comments == 0 {
			continue
		}
		var found []string
		if reviews > 0 {
			found = append(found, fmt.Sprintf("新しいレビュー %d 件（%s）", reviews, formatVerdicts(verdicts)))
		}
		if comments > 0 {
			found = append(found, fmt.Sprintf("新しいコメント %d 件", comments))
		}
		parts = append(parts, fmt.Sprintf("PR #%s に%s", pullRequests[i].Number, strings.Join(found, "と")))
	}
	return strings.Join(parts, "、"), latest
}

func formatVerdicts(verdicts map[ReviewVerdict]int) string {
	var parts []string
	for _, verdict := range []struct {
		value ReviewVerdict
		label string
	}{{VerdictChangesRequested, "Request changes"}, {VerdictApproved, "Approve"}, {VerdictCommented, "Comment"}} {
		if count := verdicts[verdict.value]; count > 0 {
			parts = append(parts, fmt.Sprintf("%s %d", verdict.label, count))
		}
	}
	return strings.Join(parts, "、")
}

func (u RequestReviewUseCase) sleep(ctx context.Context) error {
	interval := u.Interval
	if interval <= 0 {
		interval = DefaultReviewPollInterval
	}
	if u.Sleep != nil {
		return u.Sleep(ctx, interval)
	}
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (u RequestReviewUseCase) report(line string) {
	if u.Report != nil {
		u.Report(line)
	}
}
