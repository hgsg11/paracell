// Package review follows pull requests on a review provider.
package review

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/hgsg11/paracell/internal/adapter/system"
	"github.com/hgsg11/paracell/internal/domain"
	"github.com/hgsg11/paracell/internal/usecase"
)

const originRemote = "origin"

var _ usecase.ReviewPort = GitHubAdapter{}

// GitHubAdapter pushes with git and follows pull requests with the gh CLI. The
// user signs in to gh beforehand. gh reads the repository from the worktree,
// so the runner must be a system.DirRunner.
type GitHubAdapter struct {
	Runner system.Runner
	Root   string
}

func NewGitHubAdapter(runner system.Runner, root string) GitHubAdapter {
	return GitHubAdapter{Runner: runner, Root: root}
}

func (a GitHubAdapter) InspectSource(ctx context.Context, source domain.SourceResource) (usecase.SourceChanges, error) {
	worktree := a.worktreePath(source)
	status, err := a.Runner.Output(ctx, "git", "-C", worktree, "status", "--porcelain")
	if err != nil {
		return usecase.SourceChanges{}, err
	}
	count, err := a.Runner.Output(ctx, "git", "-C", worktree, "rev-list", "--count", compareBase(source.Base)+"..HEAD")
	if err != nil {
		return usecase.SourceChanges{}, err
	}
	return usecase.SourceChanges{Uncommitted: strings.TrimSpace(status) != "", Committed: strings.TrimSpace(count) != "0"}, nil
}

// Push sends the branch to origin. The branch belongs to the Cell, so a
// rewritten history is pushed as long as nobody else pushed in between.
func (a GitHubAdapter) Push(ctx context.Context, source domain.SourceResource) error {
	return a.Runner.Run(ctx, "git", "-C", a.worktreePath(source), "push", "--force-with-lease", "-u", originRemote, source.Branch)
}

func (a GitHubAdapter) OpenPullRequest(ctx context.Context, source domain.SourceResource, issue string, title string, body string) (string, error) {
	gh, err := a.gh(source)
	if err != nil {
		return "", err
	}
	open, err := gh.Output(ctx, "gh", "pr", "list", "--head", source.Branch, "--state", "open", "--json", "number", "--jq", ".[0].number")
	if err != nil {
		return "", err
	}
	if number := strings.TrimSpace(open); number != "" {
		return number, nil
	}
	args := []string{"pr", "create", "--head", source.Branch, "--title", title, "--body", withClosingReference(body, issue)}
	if base := baseBranch(source.Base); base != "" {
		args = append(args, "--base", base)
	}
	output, err := gh.Output(ctx, "gh", args...)
	if err != nil {
		return "", err
	}
	return pullRequestNumber(output)
}

func (a GitHubAdapter) PullRequest(ctx context.Context, source domain.SourceResource, number string) (usecase.PullRequestReport, error) {
	gh, err := a.gh(source)
	if err != nil {
		return usecase.PullRequestReport{}, err
	}
	output, err := gh.Output(ctx, "gh", "pr", "view", number, "--json", "state,reviews,comments")
	if err != nil {
		return usecase.PullRequestReport{}, err
	}
	return parsePullRequest(output)
}

func (a GitHubAdapter) MergePullRequest(ctx context.Context, source domain.SourceResource, number string, method usecase.MergeMethod) error {
	gh, err := a.gh(source)
	if err != nil {
		return err
	}
	switch method {
	case usecase.MergeCommit, usecase.MergeSquash, usecase.MergeRebase:
	default:
		return fmt.Errorf("unsupported merge method %q", method)
	}
	return gh.Run(ctx, "gh", "pr", "merge", number, "--"+string(method))
}

func (a GitHubAdapter) gh(source domain.SourceResource) (system.Runner, error) {
	runner, ok := a.Runner.(system.DirRunner)
	if !ok {
		return nil, fmt.Errorf("the GitHub review adapter requires a runner that can change directory")
	}
	return runner.InDir(a.worktreePath(source)), nil
}

func (a GitHubAdapter) worktreePath(source domain.SourceResource) string {
	if filepath.IsAbs(source.WorktreePath) || a.Root == "" {
		return source.WorktreePath
	}
	return filepath.Join(a.Root, source.WorktreePath)
}

// compareBase is the ref the branch is compared with to find its own commits.
// Without a configured base, the default branch of origin stands in.
func compareBase(base string) string {
	if base == "" || base == "current" {
		return originRemote + "/HEAD"
	}
	return base
}

// baseBranch is the branch name the pull request targets. An empty name lets
// GitHub use the repository's default branch.
func baseBranch(base string) string {
	if base == "" || base == "current" {
		return ""
	}
	base = strings.TrimPrefix(base, "refs/remotes/")
	base = strings.TrimPrefix(base, "refs/heads/")
	return strings.TrimPrefix(base, originRemote+"/")
}

var issueNumberPattern = regexp.MustCompile(`^#?([0-9]+)$`)

// withClosingReference appends "Closes #<issue>" unless the body already has
// it. An issue that is not a GitHub issue number gets no reference.
func withClosingReference(body string, issue string) string {
	match := issueNumberPattern.FindStringSubmatch(strings.TrimSpace(issue))
	if match == nil {
		return body
	}
	reference := "Closes #" + match[1]
	if strings.Contains(body, reference) {
		return body
	}
	body = strings.TrimRight(body, "\n")
	if body == "" {
		return reference + "\n"
	}
	return body + "\n\n" + reference + "\n"
}

var pullRequestURLPattern = regexp.MustCompile(`/pull/([0-9]+)\s*$`)

// pullRequestNumber reads the number from the URL that gh pr create prints last.
func pullRequestNumber(output string) (string, error) {
	match := pullRequestURLPattern.FindStringSubmatch(strings.TrimSpace(output))
	if match == nil {
		return "", fmt.Errorf("gh pr create did not print a pull request URL: %q", strings.TrimSpace(output))
	}
	return match[1], nil
}

type ghPullRequest struct {
	State   string `json:"state"`
	Reviews []struct {
		Author      ghAuthor   `json:"author"`
		State       string     `json:"state"`
		SubmittedAt *time.Time `json:"submittedAt"`
	} `json:"reviews"`
	Comments []struct {
		CreatedAt time.Time `json:"createdAt"`
	} `json:"comments"`
}

type ghAuthor struct {
	Login string `json:"login"`
}

func parsePullRequest(output string) (usecase.PullRequestReport, error) {
	var raw ghPullRequest
	if err := json.Unmarshal([]byte(output), &raw); err != nil {
		return usecase.PullRequestReport{}, fmt.Errorf("parse gh pr view: %w", err)
	}
	var report usecase.PullRequestReport
	switch raw.State {
	case "OPEN":
		report.Status.State = domain.PullRequestOpen
	case "MERGED":
		report.Status.State = domain.PullRequestMerged
	case "CLOSED":
		report.Status.State = domain.PullRequestClosed
	default:
		return usecase.PullRequestReport{}, fmt.Errorf("unsupported pull request state %q", raw.State)
	}
	// The latest approving or blocking review of each author stands; a
	// dismissed review withdraws it.
	standing := map[string]string{}
	for _, review := range raw.Reviews {
		if review.SubmittedAt == nil || review.State == "PENDING" {
			continue
		}
		switch review.State {
		case "APPROVED", "CHANGES_REQUESTED":
			standing[review.Author.Login] = review.State
		case "DISMISSED":
			delete(standing, review.Author.Login)
		}
		verdict := usecase.VerdictCommented
		switch review.State {
		case "APPROVED":
			verdict = usecase.VerdictApproved
		case "CHANGES_REQUESTED":
			verdict = usecase.VerdictChangesRequested
		}
		report.Events = append(report.Events, usecase.ReviewEvent{Kind: usecase.ReviewEventReview, Verdict: verdict, At: *review.SubmittedAt})
	}
	for _, comment := range raw.Comments {
		report.Events = append(report.Events, usecase.ReviewEvent{Kind: usecase.ReviewEventComment, At: comment.CreatedAt})
	}
	report.Status.Approval = domain.ApprovalNone
	for _, state := range standing {
		if state == "CHANGES_REQUESTED" {
			report.Status.Approval = domain.ApprovalChangesRequested
			break
		}
		report.Status.Approval = domain.ApprovalApproved
	}
	return report, nil
}
