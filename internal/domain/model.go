package domain

import (
	"errors"
	"fmt"
)

type Template struct {
	Name       string
	Repository RepositoryTemplate
	Containers ContainerTemplate
	Session    SessionTemplate
}

type RepositoryTemplate struct {
	BranchPrefix string `yaml:"branchPrefix" json:"branchPrefix"`
	Base         string `yaml:"base" json:"base"`
}

type ContainerTemplate struct {
	Services map[string]ContainerServiceTemplate `yaml:"services" json:"services"`
}

type ContainerServiceTemplate struct {
	SourceContainer string `yaml:"sourceContainer" json:"sourceContainer"`
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
	Branch     string
	Source     Source
	Containers Containers
	Session    Session
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
	prefix := fmt.Sprintf("pdev-%s-%s", project, name)
	services := make(map[string]CellContainer, len(template.Containers.Services))
	for role, service := range template.Containers.Services {
		services[role] = CellContainer{
			ContainerName:   fmt.Sprintf("%s-%s", prefix, role),
			SourceContainer: service.SourceContainer,
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
		Branch:   template.Repository.BranchPrefix + issue,
		Source: Source{
			Path: fmt.Sprintf(".pdev/cells/%s/source", name),
		},
		Containers: Containers{
			Network:  prefix,
			Services: services,
		},
		Session: Session{
			Name:    prefix,
			Windows: windows,
		},
	}, nil
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
