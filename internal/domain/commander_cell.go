package domain

import "fmt"

// CommanderCell is the entry point that connects the user and the AI. It refers
// to its CellGroup by ID and holds only the workspace and the execution state.
// Version is read-only for the domain: only CommanderCellFactory.Restore sets it,
// and only the state adapter advances it.
type CommanderCell struct {
	ID          string
	CellGroupID string
	Workspace   Workspace
	Status      CellStatus
	Progress    CellProgress
	Version     CellVersion
}

// NewCommanderCell creates an unsaved CommanderCell. Its Version stays zero until
// the state adapter persists it.
func NewCommanderCell(id string, cellGroupID string, workspace Workspace) (CommanderCell, error) {
	if id == "" || cellGroupID == "" {
		return CommanderCell{}, fmt.Errorf("commander cell id and CellGroup reference are required")
	}
	return CommanderCell{
		ID: id, CellGroupID: cellGroupID,
		Workspace: NewWorkspace(workspace.Driver, workspace.Windows),
		Status:    Ready, Progress: InProgress,
	}, nil
}

func (c *CommanderCell) MarkPending() {
	c.Status = Pending
}

func (c *CommanderCell) MarkReady() {
	c.Status = Ready
}

// EnsureCanBeCleaned allows cleaning only a merged or abandoned Cell.
func (c CommanderCell) EnsureCanBeCleaned() error {
	if !c.Progress.IsTerminal() {
		return fmt.Errorf("merge 済みでも中止でもないので消せない (progress: %s)", c.Progress)
	}
	return nil
}

// MarkInReview records that a PR is open and waiting for review.
func (c *CommanderCell) MarkInReview() error {
	return c.moveTo(InReview)
}

// MarkInProgress records that the Cell went back to implementation, for
// example to address review comments.
func (c *CommanderCell) MarkInProgress() error {
	return c.moveTo(InProgress)
}

// MarkMerged records that the work reached the base. It cannot be undone.
func (c *CommanderCell) MarkMerged() error {
	return c.moveTo(Merged)
}

// MarkAbandoned records that the work stopped without a merge. It cannot be undone.
func (c *CommanderCell) MarkAbandoned() error {
	return c.moveTo(Abandoned)
}

func (c *CommanderCell) moveTo(next CellProgress) error {
	progress, err := c.Progress.transitionTo(next)
	if err != nil {
		return err
	}
	c.Progress = progress
	return nil
}
