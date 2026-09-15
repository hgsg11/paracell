package domain

import (
	"bytes"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
)

// Templates is unresolved project configuration. Inheritance and runtime
// rendering intentionally happen in ResolveTemplate, not in the config adapter.
type Templates struct {
	ProjectName            string
	Templates              []Template
	SessionDriverType      SessionDriverType
	ContainerDriverType    ContainerDriverType
	SourceDriverType       SourceDriverType
	NotificationDriverType NotificationDriverType
}

func NewTemplates(projectName string, templates []Template, sessionDriverType SessionDriverType, containerDriverType ContainerDriverType, sourceDriverType SourceDriverType, notificationDriverType NotificationDriverType) (Templates, error) {
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
	return Templates{
		ProjectName: projectName, Templates: append([]Template(nil), templates...),
		SessionDriverType: sessionDriverType, ContainerDriverType: containerDriverType,
		SourceDriverType: sourceDriverType, NotificationDriverType: notificationDriverType,
	}, nil
}

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

type ResolvedTemplate struct {
	Name       string
	Sources    []SourceTemplate
	Containers []ContainerTemplate
	Session    SessionTemplate
}

func NewResolvedTemplate(name string, sources []SourceTemplate, containers []ContainerTemplate, session SessionTemplate) ResolvedTemplate {
	return ResolvedTemplate{Name: name, Sources: append([]SourceTemplate(nil), sources...), Containers: append([]ContainerTemplate(nil), containers...), Session: NewSessionTemplate(session.Windows)}
}

type TemplateVars struct {
	Issue   string
	Name    string
	Project string
	Command string
}

func NewTemplateVars(issue string, name string, project string, command string) TemplateVars {
	return TemplateVars{Issue: issue, Name: name, Project: project, Command: command}
}

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
	var containers []ContainerTemplate
	if item.Containers != nil {
		containers = append([]ContainerTemplate(nil), (*item.Containers)...)
		for i := range containers {
			for j := range containers[i].Environments {
				value, renderErr := renderTemplateValue(containers[i].Environments[j].Value, vars)
				if renderErr != nil {
					return ResolvedTemplate{}, fmt.Errorf("render environment %q for container %q: %w", containers[i].Environments[j].Name, containers[i].Name, renderErr)
				}
				containers[i].Environments[j].Value = value
			}
		}
	}
	session := NewSessionTemplate(nil)
	if item.Session != nil {
		session = NewSessionTemplate(item.Session.Windows)
		for i := range session.Windows {
			command, renderErr := renderTemplateValue(session.Windows[i].Command, vars)
			if renderErr != nil {
				return ResolvedTemplate{}, fmt.Errorf("render session window %q: %w", session.Windows[i].Name, renderErr)
			}
			session.Windows[i].Command = command
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
			item = mergeTemplate(parent, item)
		}
		path = path[:len(path)-1]
		states[current] = 2
		definitions[current] = item
		return item, nil
	}
	return resolve(name)
}

func mergeTemplate(parent Template, child Template) Template {
	merged := parent
	merged.Name, merged.Extends, merged.Abstract = child.Name, child.Extends, child.Abstract
	if child.Repository != nil {
		if parent.Repository == nil {
			merged.Repository = child.Repository
		} else {
			repository := mergeSourceTemplate(*parent.Repository, *child.Repository)
			merged.Repository = &repository
		}
	}
	if child.Containers != nil {
		merged.Containers = child.Containers
	}
	if child.Session != nil {
		merged.Session = child.Session
	}
	return merged
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

type ContainerTemplate struct {
	Name         string
	Mode         Mode
	Environments []Environment
	Mounts       []Mount
}

func NewContainerTemplate(name string, mode Mode, environments []Environment, mounts []Mount) (ContainerTemplate, error) {
	if name == "" {
		return ContainerTemplate{}, fmt.Errorf("container name is required")
	}
	validatedMode, err := NewMode(string(mode))
	if err != nil {
		return ContainerTemplate{}, err
	}
	if validatedMode == Dependency && (len(environments) != 0 || len(mounts) != 0) {
		return ContainerTemplate{}, fmt.Errorf("dependency container %q cannot define environments or mounts", name)
	}
	return ContainerTemplate{Name: name, Mode: validatedMode, Environments: append([]Environment(nil), environments...), Mounts: append([]Mount(nil), mounts...)}, nil
}

type Mode string

const (
	Target     Mode = "target"
	Dependency Mode = "dependency"
)

func NewMode(value string) (Mode, error) {
	mode := Mode(value)
	switch mode {
	case Target, Dependency:
		return mode, nil
	default:
		return mode, fmt.Errorf("invalid mode %q", mode)
	}
}

type Environment struct {
	Name  string
	Value string
}

func NewEnvironment(name string, value string) (Environment, error) {
	if name == "" {
		return Environment{}, fmt.Errorf("environment name is required")
	}
	return Environment{Name: name, Value: value}, nil
}

type Mount struct {
	TargetPath string
	SourcePath string
}

func NewMount(targetPath string, sourcePath string) (Mount, error) {
	if !filepath.IsAbs(targetPath) {
		return Mount{}, fmt.Errorf("mount target path %q must be absolute", targetPath)
	}
	if filepath.IsAbs(sourcePath) {
		return Mount{}, fmt.Errorf("mount source path %q must be relative", sourcePath)
	}
	clean := filepath.Clean(sourcePath)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return Mount{}, fmt.Errorf("mount source path %q must stay within its source root", sourcePath)
	}
	return Mount{TargetPath: targetPath, SourcePath: clean}, nil
}

type SourceTemplate struct {
	Path      string
	Base      string
	Prefix    string
	pathSet   bool
	baseSet   bool
	prefixSet bool
}

func NewSourceTemplate(path string, base string, prefix string) (SourceTemplate, error) {
	if path == "" {
		path = "."
	}
	if filepath.IsAbs(path) {
		return SourceTemplate{}, fmt.Errorf("source path %q must be relative", path)
	}
	clean := filepath.Clean(path)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return SourceTemplate{}, fmt.Errorf("source path %q must stay within project root", path)
	}
	return SourceTemplate{Path: clean, Base: base, Prefix: prefix, pathSet: true, baseSet: true, prefixSet: true}, nil
}

func NewPartialSourceTemplate(path *string, base *string, prefix *string) (SourceTemplate, error) {
	value := SourceTemplate{}
	if path != nil {
		parsed, err := NewSourceTemplate(*path, "", "")
		if err != nil {
			return SourceTemplate{}, err
		}
		value.Path, value.pathSet = parsed.Path, true
	}
	if base != nil {
		value.Base, value.baseSet = *base, true
	}
	if prefix != nil {
		value.Prefix, value.prefixSet = *prefix, true
	}
	return value, nil
}

func mergeSourceTemplate(parent SourceTemplate, child SourceTemplate) SourceTemplate {
	merged := parent
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

type SessionTemplate struct{ Windows []Window }

func NewSessionTemplate(windows []Window) SessionTemplate {
	return SessionTemplate{Windows: append([]Window(nil), windows...)}
}

type Window struct {
	Name    string
	Command string
}

func NewWindow(name string, command string) (Window, error) {
	if name == "" {
		return Window{}, fmt.Errorf("window name is required")
	}
	return Window{Name: name, Command: command}, nil
}

type ContainerDriverType string

const (
	None   ContainerDriverType = "none"
	Docker ContainerDriverType = "docker"
)

func NewContainerDriverType(value string) ContainerDriverType {
	driver := ContainerDriverType(value)
	if driver == Docker {
		return driver
	}
	return None
}

type SessionDriverType string

const Tmux SessionDriverType = "tmux"

func NewSessionDriverType(value string) (SessionDriverType, error) {
	driver := SessionDriverType(value)
	if driver == Tmux {
		return driver, nil
	}
	return driver, fmt.Errorf("invalid session driver type %q", driver)
}

type SourceDriverType string

const Git SourceDriverType = "git"

func NewSourceDriverType(value string) (SourceDriverType, error) {
	driver := SourceDriverType(value)
	if driver == Git {
		return driver, nil
	}
	return driver, fmt.Errorf("invalid source driver type %q", driver)
}

type NotificationDriverType string

const (
	NoNotification   NotificationDriverType = "none"
	TmuxNotification NotificationDriverType = "tmux"
)

func NewNotificationDriverType(value string) (NotificationDriverType, error) {
	driver := NotificationDriverType(value)
	if driver == "" {
		return NoNotification, nil
	}
	switch driver {
	case NoNotification, TmuxNotification:
		return driver, nil
	default:
		return driver, fmt.Errorf("invalid notification driver type %q", driver)
	}
}
