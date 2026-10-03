package domain

import (
	"bytes"
	"text/template"
)

type TemplateVars struct {
	Issue   string
	Name    string
	Project string
	Command string
}

func NewTemplateVars(issue string, name string, project string, command string) TemplateVars {
	return TemplateVars{Issue: issue, Name: name, Project: project, Command: command}
}

func (v TemplateVars) Render(value string) (string, error) {
	tpl, err := template.New("value").Option("missingkey=error").Parse(value)
	if err != nil {
		return "", err
	}
	var rendered bytes.Buffer
	if err := tpl.Execute(&rendered, v); err != nil {
		return "", err
	}
	return rendered.String(), nil
}
