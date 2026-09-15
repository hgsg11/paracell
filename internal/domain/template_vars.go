package domain

type TemplateVars struct {
	Issue   string
	Name    string
	Project string
	Command string
}

func NewTemplateVars(issue string, name string, project string, command string) TemplateVars {
	return TemplateVars{Issue: issue, Name: name, Project: project, Command: command}
}
