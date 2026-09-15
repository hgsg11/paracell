package domain

import "fmt"

type Template struct {
	Name       string
	Extends    string
	Abstract   bool
	Repository *SourceTemplate
	Containers *[]ContainerTemplate
	Session    *SessionTemplate
}

func NewTemplate(name string, sources []SourceTemplate, containers []ContainerTemplate, session SessionTemplate) (Template, error) {
	if len(sources) > 1 {
		return Template{}, fmt.Errorf("template %q defines more than one repository", name)
	}
	var repository *SourceTemplate
	if len(sources) == 1 {
		source := sources[0]
		repository = &source
	}
	containersCopy := append([]ContainerTemplate(nil), containers...)
	sessionCopy := session
	return NewUnresolvedTemplate(name, "", false, repository, &containersCopy, &sessionCopy)
}

func NewUnresolvedTemplate(name string, extends string, abstract bool, repository *SourceTemplate, containers *[]ContainerTemplate, session *SessionTemplate) (Template, error) {
	if name == "" {
		return Template{}, fmt.Errorf("template name is required")
	}
	if containers != nil {
		names := make(map[string]struct{}, len(*containers))
		for _, container := range *containers {
			if _, exists := names[container.Name]; exists {
				return Template{}, fmt.Errorf("duplicate container %q for template %q", container.Name, name)
			}
			names[container.Name] = struct{}{}
		}
		copy := append([]ContainerTemplate(nil), (*containers)...)
		containers = &copy
	}
	if repository != nil {
		copy := *repository
		repository = &copy
	}
	if session != nil {
		copy := NewSessionTemplate(session.Windows)
		session = &copy
	}
	return Template{Name: name, Extends: extends, Abstract: abstract, Repository: repository, Containers: containers, Session: session}, nil
}
