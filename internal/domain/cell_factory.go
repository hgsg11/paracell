package domain

import (
	"errors"
	"fmt"
)

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
		ID:         id,
		Issue:      issue,
		Name:       name,
		Template:   template.Name,
		Base:       template.Repository.Base,
		Branch:     template.Repository.BranchPrefix + issue,
		BranchMode: template.Repository.BranchMode,
		Source: Source{
			Path: fmt.Sprintf(".paracell/cells/%s/source", name),
		},
		Containers: Containers{
			Network:     prefix,
			NetworkMode: template.Containers.Network,
			Services:    services,
		},
		Session: Session{
			Name:    prefix,
			Windows: windows,
		},
		status: CellStatusReady,
		done:   false,
	}, nil
}
