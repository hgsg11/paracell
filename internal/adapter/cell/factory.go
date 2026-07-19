package cell

import (
	"errors"
	"fmt"

	"github.com/hgsg11/paracell/internal/domain"
)

type Factory struct{}

func (f Factory) NewCell(id string, issue string, template domain.Template, project string) (domain.Cell, error) {
	if id == "" {
		return domain.Cell{}, errors.New("cell id is required")
	}
	if issue == "" {
		return domain.Cell{}, errors.New("issue is required")
	}
	if template.Name == "" {
		return domain.Cell{}, errors.New("template name is required")
	}
	name := issue
	prefix := fmt.Sprintf("paracell-%s-%s", project, name)
	sessionName := fmt.Sprintf("%s-%s", project, name)
	services := make(map[string]domain.CellContainer, len(template.Containers.Services))
	for role, service := range template.Containers.Services {
		var database *domain.DatabaseConfig
		if service.Database != nil {
			database = &domain.DatabaseConfig{
				System:    service.Database.System,
				CopyMode:  service.Database.CopyMode,
				InitFiles: append([]string(nil), service.Database.InitFiles...),
			}
		}
		services[role] = domain.CellContainer{
			ContainerName:   fmt.Sprintf("%s-%s", prefix, role),
			SourceContainer: service.SourceContainer,
			VolumeMode:      service.VolumeMode,
			Database:        database,
		}
	}
	windows := make([]domain.SessionWindow, 0, len(template.Session.Windows))
	for _, window := range template.Session.Windows {
		windows = append(windows, domain.SessionWindow{Name: window.Name, Command: window.Command})
	}

	return domain.Cell{
		ID:         id,
		Issue:      issue,
		Name:       name,
		Template:   template.Name,
		Base:       template.Repository.Base,
		Branch:     template.Repository.BranchPrefix + issue,
		BranchMode: template.Repository.BranchMode,
		Source: domain.Source{
			Path: fmt.Sprintf(".paracell/cells/%s/source", name),
		},
		Containers: domain.Containers{
			Network:     prefix,
			NetworkMode: string(template.Containers.Network),
			Services:    services,
		},
		Session: domain.Session{
			Name:    sessionName,
			Windows: windows,
		},
	}, nil
}
