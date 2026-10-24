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
	Done        bool
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
		Status:    Ready,
	}, nil
}

func (c *CommanderCell) MarkPending() {
	c.Status = Pending
}

func (c *CommanderCell) MarkReady() {
	c.Status = Ready
}

func (c CommanderCell) EnsureCanBeCleaned() error {
	if !c.Done {
		return fmt.Errorf("完了済みではないので消せない")
	}
	return nil
}

func (c *CommanderCell) MarkDone() error {
	if c.Done {
		return fmt.Errorf("cell is already done")
	}
	c.Done = true
	return nil
}

func (c *CommanderCell) MarkUndone() error {
	if !c.Done {
		return fmt.Errorf("cell is not done")
	}
	c.Done = false
	return nil
}
