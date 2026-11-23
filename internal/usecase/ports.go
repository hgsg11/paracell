package usecase

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/hgsg11/paracell/internal/domain"
)

type ConfigPort interface {
	Load(ctx context.Context) (domain.Templates, error)
}

type InitConfigPort interface {
	ConfigExists(ctx context.Context) (bool, error)
	SaveConfig(ctx context.Context, cfg domain.Templates) error
}

type CellInitializer interface {
	Initialize(context.Context) error
}

// AgentHookPort installs the status hooks and the allowed commands into an
// agent's configuration, adding only what is not installed yet.
type AgentHookPort interface {
	InstallStatusHooks(ctx context.Context, hooks []domain.AgentStatusHook) error
	// AllowCommands lets the agent run each command, with any arguments,
	// without asking the user.
	AllowCommands(ctx context.Context, commands []string) error
}

type CellPort interface {
	LoadCells(context.Context) (CellSet, error)
	UpdateCells(context.Context, func(CellSet) (CellSet, error)) error
}

// CellSet is a state snapshot of CellGroups, the runtime Cells associated
// with them by CellGroup ID, the Reviews in progress, and the project's Queue.
// One UpdateCells saves all of them in the same transaction.
type CellSet struct {
	Groups       []domain.CellGroup
	Commanders   []domain.CommanderCell
	Targets      []domain.TargetCell
	Dependencies []domain.DependencyCell
	Reviews      []domain.Review
	Queue        domain.Queue
}

func NewCellSet(groups []domain.CellGroup, commanders []domain.CommanderCell, targets []domain.TargetCell, dependencies []domain.DependencyCell) CellSet {
	return CellSet{
		Groups:       append([]domain.CellGroup(nil), groups...),
		Commanders:   append([]domain.CommanderCell(nil), commanders...),
		Targets:      append([]domain.TargetCell(nil), targets...),
		Dependencies: append([]domain.DependencyCell(nil), dependencies...),
	}
}

// group returns the CellGroup with the given ID.
func (s CellSet) group(id string) (domain.CellGroup, bool) {
	for _, group := range s.Groups {
		if group.ID == id {
			return group, true
		}
	}
	return domain.CellGroup{}, false
}

// resolve finds a CommanderCell and its CellGroup by a user-facing identifier.
func (s CellSet) resolve(identifier string) (domain.CommanderCell, domain.CellGroup, error) {
	commander, ok := domain.ResolveCommanderCellService(s.Groups, s.Commanders, identifier)
	if !ok {
		return domain.CommanderCell{}, domain.CellGroup{}, fmt.Errorf("cell %q not found", identifier)
	}
	group, ok := s.group(commander.CellGroupID)
	if !ok {
		return domain.CommanderCell{}, domain.CellGroup{}, fmt.Errorf("cell %q not found", identifier)
	}
	return commander, group, nil
}

// review returns the Review of the CellGroup.
func (s CellSet) review(cellGroupID string) (domain.Review, bool) {
	for _, review := range s.Reviews {
		if review.CellGroupID == cellGroupID {
			return review, true
		}
	}
	return domain.Review{}, false
}

// replaceReview replaces the Review that has the same ID.
func (s CellSet) replaceReview(review domain.Review) error {
	for i := range s.Reviews {
		if s.Reviews[i].ID == review.ID {
			s.Reviews[i] = review
			return nil
		}
	}
	return fmt.Errorf("review %q: %w", review.ID, domain.ErrNotFound)
}

// removeReview removes the Review of the CellGroup, if there is one.
func (s *CellSet) removeReview(cellGroupID string) {
	reviews := make([]domain.Review, 0, len(s.Reviews))
	for _, review := range s.Reviews {
		if review.CellGroupID != cellGroupID {
			reviews = append(reviews, review)
		}
	}
	s.Reviews = reviews
}

// replaceCommander replaces the CommanderCell that has the same ID.
func (s CellSet) replaceCommander(commander domain.CommanderCell) error {
	for i := range s.Commanders {
		if s.Commanders[i].ID == commander.ID {
			s.Commanders[i] = commander
			return nil
		}
	}
	return fmt.Errorf("CommanderCell %q not found", commander.ID)
}

// replaceGroup replaces the CellGroup that has the same ID.
func (s CellSet) replaceGroup(group domain.CellGroup) error {
	for i := range s.Groups {
		if s.Groups[i].ID == group.ID {
			s.Groups[i] = group
			return nil
		}
	}
	return fmt.Errorf("CellGroup %q not found", group.ID)
}

type SourcePort interface {
	CreateSource(context.Context, domain.SourceResource) error
	CleanSource(context.Context, domain.SourceResource) error
}

// CellDirectoryPort manages the Cell directory apart from its worktrees. The
// Cell directory is relative to the project root.
type CellDirectoryPort interface {
	// CopyWorkspaceFiles copies the workspace paths to the same paths in the Cell directory.
	CopyWorkspaceFiles(ctx context.Context, cellDirectory string, paths []string) error
	// RemoveCellDirectory removes the Cell directory and everything left in it.
	RemoveCellDirectory(ctx context.Context, cellDirectory string) error
}

// WorkspaceFilePort looks at the files in the workspace, the directory that
// holds paracell.yaml.
type WorkspaceFilePort interface {
	// ExistingPaths returns the paths that exist on disk as files. A directory
	// counts as missing.
	ExistingPaths(ctx context.Context, paths []domain.WorkspacePath) (domain.ExistingPaths, error)
}

type ContainerPort interface {
	CreateContainers(context.Context, domain.ContainerResources) (map[string][]string, error)
	CleanContainers(context.Context, domain.ContainerResources) error
}

type WorkspacePort interface {
	CreateWorkspace(context.Context, domain.WorkspaceResource) error
	CleanWorkspace(context.Context, domain.WorkspaceResource) error
	// EnterCommand prepares the cell session and returns the command that moves this terminal to it.
	EnterCommand(context.Context, domain.WorkspaceResource) (*exec.Cmd, error)
	UpdateStatusLabel(context.Context, domain.WorkspaceResource) error
	EnterWorkspace(context.Context, domain.WorkspaceResource) error
	EnterRootWorkspace(context.Context, string) error
	ExitWorkspace(context.Context) error
}

type Notifier interface {
	NotifyReady(ctx context.Context, resource domain.WorkspaceResource, message string) error
}

type NotificationProviderFactory interface {
	Notification(driver domain.NotificationDriverType) (Notifier, error)
}

type SourceProviderFactory interface {
	Source(driver domain.SourceDriverType) (SourcePort, error)
}

type ContainerProviderFactory interface {
	Container(driver domain.ContainerDriverType) (ContainerPort, error)
}

type WorkspaceProviderFactory interface {
	Workspace(driver domain.WorkspaceDriverType) (WorkspacePort, error)
}

// SourceChanges is what a worktree holds that its pull request does not see yet.
type SourceChanges struct {
	// Uncommitted is true when the worktree has changes that are not committed.
	Uncommitted bool
	// Committed is true when the branch has commits that the base does not.
	Committed bool
}

// ReviewEventKind tells a submitted review from a plain comment.
type ReviewEventKind string

const (
	ReviewEventReview  ReviewEventKind = "review"
	ReviewEventComment ReviewEventKind = "comment"
)

// ReviewVerdict is the verdict a submitted review carries.
type ReviewVerdict string

const (
	VerdictApproved         ReviewVerdict = "approved"
	VerdictChangesRequested ReviewVerdict = "changes_requested"
	VerdictCommented        ReviewVerdict = "commented"
)

// ReviewEvent is one review or comment written on a pull request.
type ReviewEvent struct {
	Kind ReviewEventKind
	// Verdict is set only for a review.
	Verdict ReviewVerdict
	At      time.Time
}

// PullRequestReport is the current state of one pull request and every review
// and comment written on it.
type PullRequestReport struct {
	Status domain.PullRequestStatus
	Events []ReviewEvent
}

// MergeMethod is how a pull request is merged into its base.
type MergeMethod string

const (
	MergeCommit MergeMethod = "merge"
	MergeSquash MergeMethod = "squash"
	MergeRebase MergeMethod = "rebase"
)

// ReviewPort pushes the branch of a worktree and follows its pull request on
// the review provider. Provider details, such as the base branch name or the
// ticket closing syntax, stay inside the adapter.
type ReviewPort interface {
	InspectSource(ctx context.Context, source domain.SourceResource) (SourceChanges, error)
	Push(ctx context.Context, source domain.SourceResource) error
	// OpenPullRequest opens a pull request from the branch, or returns the open
	// one, and returns its number. The body gets the reference that closes issue.
	OpenPullRequest(ctx context.Context, source domain.SourceResource, issue string, title string, body string) (string, error)
	PullRequest(ctx context.Context, source domain.SourceResource, number string) (PullRequestReport, error)
	MergePullRequest(ctx context.Context, source domain.SourceResource, number string, method MergeMethod) error
}

type ReviewProviderFactory interface {
	Review(driver domain.ReviewDriverType) (ReviewPort, error)
}

type IDGenerator interface {
	NewID() string
}
