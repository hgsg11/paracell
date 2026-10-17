package domain

import (
	"fmt"
	"sort"
)

type Templates struct {
	ProjectName            string
	Templates              []Template
	WorkspaceDriverType    WorkspaceDriverType
	ContainerDriverType    ContainerDriverType
	SourceDriverType       SourceDriverType
	NotificationDriverType NotificationDriverType
	Prefixes               map[string]string
}

const (
	FeatPrefix   = "feat"
	UpdatePrefix = "update"
	FixPrefix    = "fix"
)

var defaultPrefixes = map[string]string{FeatPrefix: "feat/", UpdatePrefix: "update/", FixPrefix: "fix/"}

func (t Templates) BranchPrefix(name string) (string, error) {
	prefix, ok := t.Prefixes[name]
	if !ok {
		return "", fmt.Errorf("prefix %q not found", name)
	}
	return prefix, nil
}

func (t Templates) Resolve(name string, vars TemplateVars) (ResolvedTemplate, error) {
	item, err := t.find(name)
	if err != nil {
		return ResolvedTemplate{}, err
	}
	return item.resolve(vars)
}

func (t Templates) SelectableNames() ([]string, error) {
	names := make([]string, 0, len(t.Templates))
	for _, item := range t.Templates {
		if item.Commander == nil {
			return nil, fmt.Errorf("template %q does not define a CommanderCell", item.Name)
		}
		names = append(names, item.Name)
	}
	sort.Strings(names)
	return names, nil
}

func (t Templates) find(name string) (Template, error) {
	for _, item := range t.Templates {
		if item.Name == name {
			return item, nil
		}
	}
	return Template{}, fmt.Errorf("template %q not found", name)
}

func NewTemplates(projectName string, templates []Template, workspaceDriverType WorkspaceDriverType, containerDriverType ContainerDriverType, sourceDriverType SourceDriverType, notificationDriverType NotificationDriverType, prefixes map[string]string) (Templates, error) {
	names := make(map[string]struct{}, len(templates))
	for _, item := range templates {
		if item.Name == "" {
			return Templates{}, fmt.Errorf("template name is required")
		}
		if _, exists := names[item.Name]; exists {
			return Templates{}, fmt.Errorf("duplicate template %q", item.Name)
		}
		names[item.Name] = struct{}{}
	}
	resolvedPrefixes := make(map[string]string, len(defaultPrefixes)+len(prefixes))
	for name, value := range defaultPrefixes {
		resolvedPrefixes[name] = value
	}
	for name, value := range prefixes {
		resolvedPrefixes[name] = value
	}
	return Templates{
		ProjectName: projectName, Templates: append([]Template(nil), templates...),
		WorkspaceDriverType: workspaceDriverType, ContainerDriverType: containerDriverType, Prefixes: resolvedPrefixes,
		SourceDriverType: sourceDriverType, NotificationDriverType: notificationDriverType,
	}, nil
}
