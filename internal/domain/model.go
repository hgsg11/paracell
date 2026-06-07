package domain

import (
	"encoding/json"
	"errors"
	"fmt"
)

type Template struct {
	Name       string
	Repository RepositoryTemplate
	Files      []string
	Containers ContainerTemplate
	Session    SessionTemplate
}

type TemplateVars struct {
	Issue string
	Name  string
}

type RepositoryTemplate struct {
	BranchPrefix string `yaml:"branchPrefix" json:"branchPrefix"`
	Base         string `yaml:"base" json:"base"`
}

type ContainerTemplate struct {
	Services map[string]ContainerServiceTemplate `yaml:"services" json:"services"`
}

type DatabaseConfig struct {
	System    string   `yaml:"system,omitempty" json:"system,omitempty"`
	CopyMode  string   `yaml:"copyMode,omitempty" json:"copyMode,omitempty"`
	InitFiles []string `yaml:"initFiles,omitempty" json:"initFiles,omitempty"`
}

type ContainerServiceTemplate struct {
	SourceContainer string          `yaml:"sourceContainer" json:"sourceContainer"`
	VolumeMode      string          `yaml:"volumeMode,omitempty" json:"volumeMode,omitempty"`
	Database        *DatabaseConfig `yaml:"database,omitempty" json:"database,omitempty"`
}

type SessionTemplate struct {
	Windows []SessionWindowTemplate `yaml:"windows" json:"windows"`
}

type SessionWindowTemplate struct {
	Name    string `yaml:"name" json:"name"`
	Command string `yaml:"command" json:"command"`
}

type Cell struct {
	ID         string
	Issue      string
	Name       string
	Template   string
	Base       string
	Branch     string
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
	Network  string
	Services map[string]CellContainer
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

type CellFactory struct{}

func NewCellFactory() CellFactory {
	return CellFactory{}
}

func (f CellFactory) NewCell(id string, issue string, template Template, project string) (Cell, error) {
	if id == "" {
		return Cell{}, errors.New("cell id is required")
	}
	if issue == "" {
		return Cell{}, errors.New("issue is required")
	}
	if template.Name == "" {
		return Cell{}, errors.New("template name is required")
	}
	name := issue
	prefix := fmt.Sprintf("paracell-%s-%s", project, name)
	services := make(map[string]CellContainer, len(template.Containers.Services))
	for role, service := range template.Containers.Services {
		var database *DatabaseConfig
		if service.Database != nil {
			database = &DatabaseConfig{
				System:    service.Database.System,
				CopyMode:  service.Database.CopyMode,
				InitFiles: append([]string(nil), service.Database.InitFiles...),
			}
		}
		services[role] = CellContainer{
			ContainerName:   fmt.Sprintf("%s-%s", prefix, role),
			SourceContainer: service.SourceContainer,
			VolumeMode:      service.VolumeMode,
			Database:        database,
		}
	}
	windows := make([]SessionWindow, 0, len(template.Session.Windows))
	for _, window := range template.Session.Windows {
		windows = append(windows, SessionWindow{Name: window.Name, Command: window.Command})
	}

	return Cell{
		ID:       id,
		Issue:    issue,
		Name:     name,
		Template: template.Name,
		Base:     template.Repository.Base,
		Branch:   template.Repository.BranchPrefix + issue,
		Source: Source{
			Path: fmt.Sprintf(".paracell/cells/%s/source", name),
		},
		Containers: Containers{
			Network:  prefix,
			Services: services,
		},
		Session: Session{
			Name:    prefix,
			Windows: windows,
		},
		status: CellStatusReady,
		done:   false,
	}, nil
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

type CellUniquenessChecker struct{}

func (c CellUniquenessChecker) EnsureUnique(existing []Cell, issue string, name string) error {
	for _, cell := range existing {
		if cell.Issue == issue {
			return fmt.Errorf("cell issue %q already exists", issue)
		}
		if cell.Name == name {
			return fmt.Errorf("cell name %q already exists", name)
		}
	}
	return nil
}
