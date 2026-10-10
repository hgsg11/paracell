package domain

import (
	"fmt"
	"sort"
	"strings"
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
	FeatPrefix = "feat"
	FixPrefix  = "fix"
)

var defaultPrefixes = map[string]string{FeatPrefix: "feat/", FixPrefix: "fix/"}

func (t Templates) BranchPrefix(name string) (string, error) {
	prefix, ok := t.Prefixes[name]
	if !ok {
		return "", fmt.Errorf("prefix %q not found", name)
	}
	return prefix, nil
}

func (t Templates) Resolve(name string, vars TemplateVars) (ResolvedTemplate, error) {
	item, err := t.resolveDefinition(name)
	if err != nil {
		return ResolvedTemplate{}, err
	}
	if item.Abstract {
		return ResolvedTemplate{}, fmt.Errorf("template %q is abstract", name)
	}
	return item.resolve(vars)
}

func (t Templates) SelectableNames() ([]string, error) {
	names := make([]string, 0, len(t.Templates))
	for _, item := range t.Templates {
		if item.Abstract {
			continue
		}
		resolved, err := t.resolveDefinition(item.Name)
		if err != nil {
			return nil, err
		}
		if resolved.Commander == nil {
			return nil, fmt.Errorf("template %q does not define a CommanderCell", resolved.Name)
		}
		names = append(names, item.Name)
	}
	sort.Strings(names)
	return names, nil
}

func (t Templates) resolveDefinition(name string) (Template, error) {
	definitions := make(map[string]Template, len(t.Templates))
	for _, item := range t.Templates {
		definitions[item.Name] = item
	}
	states := make(map[string]uint8, len(definitions))
	path := make([]string, 0, len(definitions))
	var resolve func(string) (Template, error)
	resolve = func(current string) (Template, error) {
		item, exists := definitions[current]
		if !exists {
			return Template{}, fmt.Errorf("template %q not found", current)
		}
		switch states[current] {
		case 2:
			return item, nil
		case 1:
			start := 0
			for i, entry := range path {
				if entry == current {
					start = i
					break
				}
			}
			cycle := append(append([]string(nil), path[start:]...), current)
			quoted := make([]string, len(cycle))
			for i, entry := range cycle {
				quoted[i] = fmt.Sprintf("%q", entry)
			}
			return Template{}, fmt.Errorf("template inheritance cycle: %s", strings.Join(quoted, " -> "))
		}
		states[current] = 1
		path = append(path, current)
		if item.Extends != "" {
			if _, exists := definitions[item.Extends]; !exists {
				return Template{}, fmt.Errorf("template %q extends unknown template %q", current, item.Extends)
			}
			parent, err := resolve(item.Extends)
			if err != nil {
				return Template{}, err
			}
			item, err = item.merge(parent)
			if err != nil {
				return Template{}, err
			}
		}
		path = path[:len(path)-1]
		states[current] = 2
		definitions[current] = item
		return item, nil
	}
	return resolve(name)
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
