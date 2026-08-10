package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode"
)

type Cell struct {
	ID         string
	Issue      string
	Name       string
	Note       string
	Template   string
	Base       string
	Branch     string
	BranchMode string
	Source     Source
	Containers Containers
	Session    Session
	Creation   CellCreation
	status     CellStatus
	done       bool
}

type CreationStatus string

const (
	CreationCreating CreationStatus = "creating"
	CreationFailed   CreationStatus = "failed"
	CreationReady    CreationStatus = "ready"
)

type CreationStage string

const (
	CreationStageSource     CreationStage = "source"
	CreationStageFiles      CreationStage = "files"
	CreationStageContainers CreationStage = "containers"
	CreationStageSession    CreationStage = "session"
)

type CellCreation struct {
	Status          CreationStatus  `json:"status"`
	Command         string          `json:"command,omitempty"`
	CompletedStages []CreationStage `json:"completedStages,omitempty"`
	FailedStage     CreationStage   `json:"failedStage,omitempty"`
	LastError       string          `json:"lastError,omitempty"`
}

type CellStatus string

const (
	Pending CellStatus = "pending"
	Ready   CellStatus = "ready"
)

func (c Cell) MarshalJSON() ([]byte, error) {
	type cellJSON struct {
		ID         string       `json:"id"`
		Issue      string       `json:"issue"`
		Name       string       `json:"name"`
		Note       string       `json:"note,omitempty"`
		Template   string       `json:"template"`
		Base       string       `json:"base"`
		Branch     string       `json:"branch"`
		BranchMode string       `json:"branchMode,omitempty"`
		Source     Source       `json:"source"`
		Containers Containers   `json:"containers"`
		Session    Session      `json:"session"`
		Creation   CellCreation `json:"creation,omitempty"`
		Status     CellStatus   `json:"status"`
		Done       bool         `json:"done"`
	}
	return json.Marshal(cellJSON{
		ID:         c.ID,
		Issue:      c.Issue,
		Name:       c.Name,
		Note:       c.Note,
		Template:   c.Template,
		Base:       c.Base,
		Branch:     c.Branch,
		BranchMode: c.BranchMode,
		Source:     c.Source,
		Containers: c.Containers,
		Session:    c.Session,
		Creation:   c.Creation,
		Status:     c.Status(),
		Done:       c.done,
	})
}

func (c *Cell) UnmarshalJSON(data []byte) error {
	type cellJSON struct {
		ID         string       `json:"id"`
		Issue      string       `json:"issue"`
		Name       string       `json:"name"`
		Note       string       `json:"note,omitempty"`
		Template   string       `json:"template"`
		Base       string       `json:"base"`
		Branch     string       `json:"branch"`
		BranchMode string       `json:"branchMode,omitempty"`
		Source     Source       `json:"source"`
		Containers Containers   `json:"containers"`
		Session    Session      `json:"session"`
		Creation   CellCreation `json:"creation,omitempty"`
		Status     CellStatus   `json:"status"`
		Done       bool         `json:"done"`
	}
	var decoded cellJSON
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	c.ID = decoded.ID
	c.Issue = decoded.Issue
	c.Name = decoded.Name
	c.Note = decoded.Note
	c.Template = decoded.Template
	c.Base = decoded.Base
	c.Branch = decoded.Branch
	c.BranchMode = decoded.BranchMode
	c.Source = decoded.Source
	c.Containers = decoded.Containers
	c.Session = decoded.Session
	c.Creation = decoded.Creation
	if decoded.Status == "" {
		c.status = Ready
	} else {
		c.status = decoded.Status
	}
	c.done = decoded.Done
	return nil
}

func (c *Cell) BeginCreation(command string) {
	c.Creation = CellCreation{Status: CreationCreating, Command: command}
}

func (c *Cell) ResumeCreation() {
	c.Creation.Status = CreationCreating
	c.Creation.FailedStage = ""
	c.Creation.LastError = ""
}

func (c *Cell) CompleteCreationStage(stage CreationStage) {
	if !c.CreationStageCompleted(stage) {
		c.Creation.CompletedStages = append(c.Creation.CompletedStages, stage)
	}
	c.Creation.FailedStage = ""
	c.Creation.LastError = ""
}

func (c *Cell) FailCreation(stage CreationStage, err error) {
	c.Creation.Status = CreationFailed
	c.Creation.FailedStage = stage
	if err == nil {
		c.Creation.LastError = ""
		return
	}
	c.Creation.LastError = err.Error()
}

func (c *Cell) FinishCreation() {
	c.Creation.Status = CreationReady
	c.Creation.FailedStage = ""
	c.Creation.LastError = ""
}

func (c Cell) CreationStatus() CreationStatus {
	if c.Creation.Status == "" {
		return CreationReady
	}
	return c.Creation.Status
}

func (c Cell) CreationStageCompleted(stage CreationStage) bool {
	for _, completed := range c.Creation.CompletedStages {
		if completed == stage {
			return true
		}
	}
	return false
}

func NormalizeCellNote(note string) (string, error) {
	normalized := strings.Join(strings.FieldsFunc(note, unicode.IsSpace), " ")
	length := len([]rune(normalized))
	if length == 0 || length > 20 {
		return "", fmt.Errorf("cell note must be between 1 and 20 characters after whitespace normalization")
	}
	return normalized, nil
}

func (c *Cell) SetNote(note string) error {
	normalized, err := NormalizeCellNote(note)
	if err != nil {
		return err
	}
	c.Note = normalized
	return nil
}

func (c Cell) DisplayLabel() string {
	if c.Note != "" {
		return c.Note
	}
	return c.Name
}

func (c Cell) TUIDisplayLabel() string {
	if c.Note != "" {
		return c.Name + " | " + c.Note
	}
	return c.Name
}

type Source struct {
	Path string
}

type Containers struct {
	Network     string
	NetworkMode string
	Services    map[string]CellContainer
}

type CellContainer struct {
	ContainerName   string
	SourceContainer string
	VolumeMode      string
	Database        *DatabaseConfig
}

func (c *CellContainer) Rename(name string) error {
	if name == "" {
		return errors.New("container name is required")
	}
	c.ContainerName = name
	return nil
}

type Session struct {
	Name    string
	Windows []SessionWindow
}

type SessionWindow struct {
	Name    string
	Command string
}

func (c *Cell) RenameContainer(role string, name string) error {
	service, ok := c.Containers.Services[role]
	if !ok {
		return fmt.Errorf("container service role %q not found", role)
	}
	if err := service.Rename(name); err != nil {
		return err
	}
	c.Containers.Services[role] = service
	return nil
}

func (c *Cell) MarkDone() error {
	if c.done {
		return fmt.Errorf("cell is already done")
	}
	c.done = true
	return nil
}

func (c *Cell) ToggleDone() {
	c.done = !c.done
}

func (c *Cell) SetStatus(status CellStatus) error {
	switch status {
	case Pending, Ready:
		c.status = status
		return nil
	default:
		return fmt.Errorf("unsupported status %q", status)
	}
}

func (c Cell) Status() CellStatus {
	if c.status == "" {
		return Ready
	}
	return c.status
}

func (c Cell) IsDone() bool {
	return c.done
}

func (c *Cell) Clean() error {
	if !c.done {
		return fmt.Errorf("完了済みではないので消せない")
	}
	return nil
}
