package domain

import (
	"bytes"
	"fmt"
	"text/template"
)

func renderContainerTemplates(templates *[]ContainerTemplate, vars TemplateVars) ([]ContainerTemplate, error) {
	if templates == nil {
		return nil, nil
	}
	containers := make([]ContainerTemplate, 0, len(*templates))
	for _, container := range *templates {
		environments := make([]Environment, 0, len(container.Environments))
		for _, environment := range container.Environments {
			value, err := renderTemplateValue(environment.Value, vars)
			if err != nil {
				return nil, fmt.Errorf("render environment %q for container %q: %w", environment.Name, container.Name, err)
			}
			rendered, err := NewEnvironment(environment.Name, value)
			if err != nil {
				return nil, err
			}
			environments = append(environments, rendered)
		}
		rendered, err := NewContainerTemplate(container.Name, container.Mode, environments, container.Mounts)
		if err != nil {
			return nil, err
		}
		containers = append(containers, rendered)
	}
	return containers, nil
}

func renderSessionTemplate(session SessionTemplate, vars TemplateVars) (SessionTemplate, error) {
	windows := make([]Window, 0, len(session.Windows))
	for _, window := range session.Windows {
		command, err := renderTemplateValue(window.Command, vars)
		if err != nil {
			return SessionTemplate{}, fmt.Errorf("render session window %q: %w", window.Name, err)
		}
		rendered, err := NewWindow(window.Name, command)
		if err != nil {
			return SessionTemplate{}, err
		}
		windows = append(windows, rendered)
	}
	return NewSessionTemplate(windows), nil
}

func renderTemplateValue(value string, vars TemplateVars) (string, error) {
	tpl, err := template.New("value").Option("missingkey=error").Parse(value)
	if err != nil {
		return "", err
	}
	var rendered bytes.Buffer
	if err := tpl.Execute(&rendered, vars); err != nil {
		return "", err
	}
	return rendered.String(), nil
}
