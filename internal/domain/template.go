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

func (t Template) resolve(vars TemplateVars) (ResolvedTemplate, error) {
	var sources []SourceTemplate
	if t.Repository != nil {
		sources = []SourceTemplate{*t.Repository}
	}
	var containers []ContainerTemplate
	if t.Containers != nil {
		containers = make([]ContainerTemplate, 0, len(*t.Containers))
		for _, container := range *t.Containers {
			rendered, err := container.render(vars)
			if err != nil {
				return ResolvedTemplate{}, err
			}
			containers = append(containers, rendered)
		}
	}
	session := NewSessionTemplate(nil)
	if t.Session != nil {
		var err error
		session, err = t.Session.render(vars)
		if err != nil {
			return ResolvedTemplate{}, err
		}
	}
	return NewResolvedTemplate(t.Name, sources, containers, session), nil
}

func (t Template) merge(parent Template) (Template, error) {
	repository := parent.Repository
	if t.Repository != nil {
		if parent.Repository == nil {
			repository = t.Repository
		} else {
			merged, err := t.Repository.merge(*parent.Repository)
			if err != nil {
				return Template{}, err
			}
			repository = &merged
		}
	}
	containers := parent.Containers
	if t.Containers != nil {
		containers = t.Containers
	}
	session := parent.Session
	if t.Session != nil {
		session = t.Session
	}
	return NewUnresolvedTemplate(t.Name, t.Extends, t.Abstract, repository, containers, session)
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
