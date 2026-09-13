package domain

import (
	"fmt"
	"path/filepath"
	"strings"
)

type TemplatesInterface interface {
	GetContainerTemplates(templateName string) ([]ContainerTemplate, error)
	GetSessionTemplate(templateName string) (SessionTemplate, error)
	GetSourceTemplates(templateName string) ([]SourceTemplate, error)
	GetContainerDriverType() ContainerDriverType
	GetSourceDriverType() SourceDriverType
	GetSessionDriverType() SessionDriverType
	GetNotificationDriverType() NotificationDriverType
}

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
	for _, template := range templates {
		if template.Name == "" {
			return Templates{}, fmt.Errorf("template name is required")
		}
		if _, exists := names[template.Name]; exists {
			return Templates{}, fmt.Errorf("duplicate template %q", template.Name)
		}
		names[template.Name] = struct{}{}
	}
	return Templates{
		ProjectName:            projectName,
		Templates:              append([]Template(nil), templates...),
		SessionDriverType:      sessionDriverType,
		ContainerDriverType:    containerDriverType,
		SourceDriverType:       sourceDriverType,
		NotificationDriverType: notificationDriverType,
	}, nil
}

func (t Templates) GetContainerTemplates(templateName string) ([]ContainerTemplate, error) {
	template, err := t.template(templateName)
	if err != nil {
		return nil, err
	}
	return append([]ContainerTemplate(nil), template.Containers...), nil
}

func (t Templates) GetSessionTemplate(templateName string) (SessionTemplate, error) {
	template, err := t.template(templateName)
	if err != nil {
		return SessionTemplate{}, err
	}
	return template.Session, nil
}

func (t Templates) GetSourceTemplates(templateName string) ([]SourceTemplate, error) {
	template, err := t.template(templateName)
	if err != nil {
		return nil, err
	}
	return append([]SourceTemplate(nil), template.Sources...), nil
}

func (t Templates) GetContainerDriverType() ContainerDriverType {
	return t.ContainerDriverType
}

func (t Templates) GetSourceDriverType() SourceDriverType {
	return t.SourceDriverType
}

func (t Templates) GetSessionDriverType() SessionDriverType {
	return t.SessionDriverType
}

func (t Templates) GetNotificationDriverType() NotificationDriverType {
	return t.NotificationDriverType
}

func (t Templates) template(name string) (Template, error) {
	for _, template := range t.Templates {
		if template.Name == name {
			return template, nil
		}
	}
	return Template{}, fmt.Errorf("template %q not found", name)
}

type Template struct {
	Name       string
	Sources    []SourceTemplate
	Containers []ContainerTemplate
	Session    SessionTemplate
}

func NewTemplate(name string, sources []SourceTemplate, containers []ContainerTemplate, session SessionTemplate) (Template, error) {
	if name == "" {
		return Template{}, fmt.Errorf("template name is required")
	}
	paths := make(map[string]struct{}, len(sources))
	for _, source := range sources {
		if _, exists := paths[source.Path]; exists {
			return Template{}, fmt.Errorf("duplicate source path %q for template %q", source.Path, name)
		}
		paths[source.Path] = struct{}{}
	}
	names := make(map[string]struct{}, len(containers))
	for _, container := range containers {
		if _, exists := names[container.Name]; exists {
			return Template{}, fmt.Errorf("duplicate container %q for template %q", container.Name, name)
		}
		names[container.Name] = struct{}{}
	}
	return Template{Name: name, Sources: append([]SourceTemplate(nil), sources...), Containers: append([]ContainerTemplate(nil), containers...), Session: session}, nil
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
	if mode == Dependency && (len(environments) != 0 || len(mounts) != 0) {
		return ContainerTemplate{}, fmt.Errorf("dependency container %q cannot define environments or mounts", name)
	}
	return ContainerTemplate{Name: name, Mode: mode, Environments: append([]Environment(nil), environments...), Mounts: append([]Mount(nil), mounts...)}, nil
}

type Mode string

const (
	Target     = Mode("target")
	Dependency = Mode("dependency")
)

func NewMode(mode string) (Mode, error) {
	m := Mode(mode)
	switch m {
	case Target, Dependency:
		return m, nil
	default:
		return m, fmt.Errorf("invalid mode %q", m)
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
	Path   string
	Base   string
	Prefix string
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
	return SourceTemplate{Path: clean, Base: base, Prefix: prefix}, nil
}

type SessionTemplate struct {
	Windows []Window
}

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
	None   = ContainerDriverType("none")
	Docker = ContainerDriverType("docker")
)

func NewContainerDriverType(driver string) ContainerDriverType {
	driverType := ContainerDriverType(driver)
	switch driverType {
	case Docker:
		return driverType
	default:
		return None
	}
}

type SessionDriverType string

const Tmux = SessionDriverType("tmux")

func NewSessionDriverType(driver string) (SessionDriverType, error) {
	driverType := SessionDriverType(driver)
	switch driverType {
	case Tmux:
		return driverType, nil
	default:
		return driverType, fmt.Errorf("invalid session driver type %q", driverType)
	}
}

type SourceDriverType string

const Git = SourceDriverType("git")

func NewSourceDriverType(driver string) (SourceDriverType, error) {
	driverType := SourceDriverType(driver)
	switch driverType {
	case Git:
		return driverType, nil
	default:
		return driverType, fmt.Errorf("invalid source driver type %q", driverType)
	}
}

type NotificationDriverType string

const (
	NoNotification   = NotificationDriverType("none")
	TmuxNotification = NotificationDriverType("tmux")
)

func NewNotificationDriverType(driver string) (NotificationDriverType, error) {
	driverType := NotificationDriverType(driver)
	if driverType == "" {
		return NoNotification, nil
	}
	switch driverType {
	case NoNotification, TmuxNotification:
		return driverType, nil
	default:
		return driverType, fmt.Errorf("invalid notification driver type %q", driverType)
	}
}

type TemplateVars struct {
	Issue   string
	Name    string
	Project string
	Command string
}
