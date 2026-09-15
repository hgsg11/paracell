package domain

import (
	"fmt"
	"sort"
	"strings"
)

func ResolveTemplate(config Templates, name string, vars TemplateVars) (ResolvedTemplate, error) {
	item, err := resolveTemplateDefinition(config, name)
	if err != nil {
		return ResolvedTemplate{}, err
	}
	if item.Abstract {
		return ResolvedTemplate{}, fmt.Errorf("template %q is abstract", name)
	}
	var sources []SourceTemplate
	if item.Repository != nil {
		sources = []SourceTemplate{*item.Repository}
	}
	containers, err := renderContainerTemplates(item.Containers, vars)
	if err != nil {
		return ResolvedTemplate{}, err
	}
	session := NewSessionTemplate(nil)
	if item.Session != nil {
		session, err = renderSessionTemplate(*item.Session, vars)
		if err != nil {
			return ResolvedTemplate{}, err
		}
	}
	return NewResolvedTemplate(name, sources, containers, session), nil
}

func SelectableTemplateNames(config Templates) ([]string, error) {
	names := make([]string, 0, len(config.Templates))
	for _, item := range config.Templates {
		if item.Abstract {
			continue
		}
		if _, err := resolveTemplateDefinition(config, item.Name); err != nil {
			return nil, err
		}
		names = append(names, item.Name)
	}
	sort.Strings(names)
	return names, nil
}

func resolveTemplateDefinition(config Templates, name string) (Template, error) {
	definitions := make(map[string]Template, len(config.Templates))
	for _, item := range config.Templates {
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
			item, err = mergeTemplate(parent, item)
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

func mergeTemplate(parent Template, child Template) (Template, error) {
	repository := parent.Repository
	if child.Repository != nil {
		if parent.Repository == nil {
			repository = child.Repository
		} else {
			merged := mergeSourceTemplate(*parent.Repository, *child.Repository)
			repository = &merged
		}
	}
	containers := parent.Containers
	if child.Containers != nil {
		containers = child.Containers
	}
	session := parent.Session
	if child.Session != nil {
		session = child.Session
	}
	return NewUnresolvedTemplate(child.Name, child.Extends, child.Abstract, repository, containers, session)
}

func mergeSourceTemplate(parent SourceTemplate, child SourceTemplate) SourceTemplate {
	merged := SourceTemplate{
		Path: parent.Path, Base: parent.Base, Prefix: parent.Prefix,
		pathSet: parent.pathSet, baseSet: parent.baseSet, prefixSet: parent.prefixSet,
	}
	if child.pathSet {
		merged.Path, merged.pathSet = child.Path, true
	}
	if child.baseSet {
		merged.Base, merged.baseSet = child.Base, true
	}
	if child.prefixSet {
		merged.Prefix, merged.prefixSet = child.Prefix, true
	}
	return merged
}
