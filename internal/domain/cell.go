package domain

import (
	"encoding/json"
	"errors"
	"fmt"
)

type Cell struct {
	ID         string
	Issue      string
	Name       string
	Template   string
	Base       string
	Branch     string
	BranchMode string
	Source     Source
	Containers Containers
	Session    Session
	status     string
	done       bool
}

const (
	CellStatusPending = "pending"
	CellStatusReady   = "ready"
)

func (c Cell) MarshalJSON() ([]byte, error) {
	type cellJSON struct {
		ID         string     `json:"id"`
		Issue      string     `json:"issue"`
		Name       string     `json:"name"`
		Template   string     `json:"template"`
		Base       string     `json:"base"`
		Branch     string     `json:"branch"`
		BranchMode string     `json:"branchMode,omitempty"`
		Source     Source     `json:"source"`
		Containers Containers `json:"containers"`
		Session    Session    `json:"session"`
		Status     string     `json:"status"`
		Done       bool       `json:"done"`
	}
	return json.Marshal(cellJSON{
		ID:         c.ID,
		Issue:      c.Issue,
		Name:       c.Name,
		Template:   c.Template,
		Base:       c.Base,
		Branch:     c.Branch,
		BranchMode: c.BranchMode,
		Source:     c.Source,
		Containers: c.Containers,
		Session:    c.Session,
		Status:     c.Status(),
		Done:       c.done,
	})
}

func (c *Cell) UnmarshalJSON(data []byte) error {
	type cellJSON struct {
		ID         string     `json:"id"`
		Issue      string     `json:"issue"`
		Name       string     `json:"name"`
		Template   string     `json:"template"`
		Base       string     `json:"base"`
		Branch     string     `json:"branch"`
		BranchMode string     `json:"branchMode,omitempty"`
		Source     Source     `json:"source"`
		Containers Containers `json:"containers"`
		Session    Session    `json:"session"`
		Status     string     `json:"status"`
		Done       bool       `json:"done"`
	}
	var decoded cellJSON
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	c.ID = decoded.ID
	c.Issue = decoded.Issue
	c.Name = decoded.Name
	c.Template = decoded.Template
	c.Base = decoded.Base
	c.Branch = decoded.Branch
	c.BranchMode = decoded.BranchMode
	c.Source = decoded.Source
	c.Containers = decoded.Containers
	c.Session = decoded.Session
	if decoded.Status == "" {
		c.status = CellStatusReady
	} else {
		c.status = decoded.Status
	}
	c.done = decoded.Done
	return nil
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

func (c *Cell) SetStatus(status string) error {
	switch status {
	case CellStatusPending, CellStatusReady:
		c.status = status
		return nil
	default:
		return fmt.Errorf("unsupported status %q", status)
	}
}

func (c Cell) Status() string {
	if c.status == "" {
		return CellStatusReady
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
