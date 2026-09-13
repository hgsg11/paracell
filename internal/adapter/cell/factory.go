package cell

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/hgsg11/paracell/internal/domain"
)

type Factory struct{}

func (f Factory) NewCell(id string, issue string, templateName string, sourceTemplates []domain.SourceTemplate, containerTemplates []domain.ContainerTemplate, sessionTemplate domain.SessionTemplate, project string) (domain.Cell, error) {
	if id == "" {
		return domain.Cell{}, errors.New("cell id is required")
	}
	if issue == "" {
		return domain.Cell{}, errors.New("issue is required")
	}
	if templateName == "" {
		return domain.Cell{}, errors.New("template name is required")
	}
	name := domain.SafeResourceName(issue, id)
	projectName := domain.SafeResourceName(project, "project")
	prefix := fmt.Sprintf("paracell-%s-%s", projectName, name)
	sources := make([]domain.Source, 0, len(sourceTemplates))
	for _, sourceTemplate := range sourceTemplates {
		path := filepath.Join(".paracell", "cells", name, "source")
		if sourceTemplate.Path != "." {
			path = filepath.Join(path, sourceTemplate.Path)
		}
		sources = append(sources, domain.Source{
			TemplatePath: sourceTemplate.Path,
			Path:         path,
			Base:         sourceTemplate.Base,
			Branch:       sourceTemplate.Prefix + issue,
		})
	}
	services := make(map[string]domain.CellContainer, len(containerTemplates))
	for _, container := range containerTemplates {
		containerName := container.Name
		if container.Mode == domain.Target {
			containerName = fmt.Sprintf("%s-%s", prefix, domain.SafeResourceName(container.Name, "container"))
		}
		services[container.Name] = domain.CellContainer{
			ContainerName:   containerName,
			SourceContainer: container.Name,
			Mode:            container.Mode,
		}
	}
	windows := make([]domain.SessionWindow, 0, len(sessionTemplate.Windows))
	for _, window := range sessionTemplate.Windows {
		windows = append(windows, domain.SessionWindow{Name: window.Name, Command: window.Command})
	}
	return domain.Cell{
		ID:         id,
		Issue:      issue,
		Name:       name,
		Template:   templateName,
		Sources:    sources,
		Containers: domain.Containers{Network: prefix, Services: services},
		Session:    domain.Session{Name: projectName + "-" + name, Windows: windows},
	}, nil
}
