package config

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"text/template"

	"github.com/hgsg11/paracell/internal/domain"
	"gopkg.in/yaml.v3"
)

type YAMLConfigAdapter struct {
	Path string
}

type yamlProviders struct {
	Source        string `yaml:"source"`
	Container     string `yaml:"container,omitempty"`
	Session       string `yaml:"session"`
	Notifications string `yaml:"notifications,omitempty"`
}

type yamlConfig struct {
	Project struct {
		Name string `yaml:"name"`
	} `yaml:"project"`
	Providers yamlProviders              `yaml:"providers"`
	Templates map[string]rawYAMLTemplate `yaml:"templates"`
}

type rawYAMLTemplate struct {
	Extends    string                 `yaml:"extends,omitempty"`
	Abstract   bool                   `yaml:"abstract,omitempty"`
	Repository *rawRepositoryTemplate `yaml:"repository,omitempty"`
	Containers *rawContainerTemplate  `yaml:"containers,omitempty"`
	Session    *rawSessionTemplate    `yaml:"session,omitempty"`
}

type rawRepositoryTemplate struct {
	Path   *string `yaml:"path,omitempty"`
	Base   *string `yaml:"base,omitempty"`
	Prefix *string `yaml:"branchPrefix,omitempty"`
}

type rawContainerTemplate struct {
	Services *map[string]rawContainer `yaml:"services,omitempty"`
}

type rawContainer struct {
	Mode        *string           `yaml:"mode,omitempty"`
	Environment map[string]string `yaml:"environment,omitempty"`
	Files       map[string]string `yaml:"files,omitempty"`
}

type rawSessionTemplate struct {
	Windows *[]rawWindow `yaml:"windows,omitempty"`
}

type rawWindow struct {
	Name    string `yaml:"name"`
	Command string `yaml:"command"`
}

func (a YAMLConfigAdapter) Load(ctx context.Context, vars *domain.TemplateVars) (domain.Templates, error) {
	_ = ctx
	data, err := os.ReadFile(a.Path)
	if err != nil {
		return domain.Templates{}, err
	}
	var raw yamlConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return domain.Templates{}, err
	}
	resolved, err := resolveTemplates(raw.Templates)
	if err != nil {
		return domain.Templates{}, err
	}
	if raw.Providers.Source == "" {
		return domain.Templates{}, fmt.Errorf("providers.source is required")
	}
	sourceDriver, err := domain.NewSourceDriverType(raw.Providers.Source)
	if err != nil {
		return domain.Templates{}, err
	}
	sessionDriver, err := domain.NewSessionDriverType(raw.Providers.Session)
	if err != nil {
		return domain.Templates{}, err
	}
	notificationDriver, err := domain.NewNotificationDriverType(raw.Providers.Notifications)
	if err != nil {
		return domain.Templates{}, err
	}
	templateVars := vars
	if vars != nil {
		copied := *vars
		copied.Project = raw.Project.Name
		templateVars = &copied
	}
	items := make([]domain.Template, 0, len(resolved))
	for _, name := range sortedMapKeys(resolved) {
		item := resolved[name]
		if item.Abstract {
			continue
		}
		domainTemplate, err := item.toDomain(name, templateVars)
		if err != nil {
			return domain.Templates{}, err
		}
		items = append(items, domainTemplate)
	}
	templates, err := domain.NewTemplates(raw.Project.Name, items, sessionDriver, domain.NewContainerDriverType(raw.Providers.Container), sourceDriver, notificationDriver)
	if err != nil {
		return domain.Templates{}, err
	}
	return templates, nil
}

type templateVisitState uint8

const (
	templateUnvisited templateVisitState = iota
	templateVisiting
	templateResolved
)

func resolveTemplates(templates map[string]rawYAMLTemplate) (map[string]rawYAMLTemplate, error) {
	resolved := make(map[string]rawYAMLTemplate, len(templates))
	states := make(map[string]templateVisitState, len(templates))
	path := make([]string, 0, len(templates))
	var resolve func(string) (rawYAMLTemplate, error)
	resolve = func(name string) (rawYAMLTemplate, error) {
		switch states[name] {
		case templateResolved:
			return resolved[name], nil
		case templateVisiting:
			return rawYAMLTemplate{}, fmt.Errorf("template inheritance cycle at %q", name)
		}
		child := templates[name]
		states[name] = templateVisiting
		path = append(path, name)
		if child.Extends != "" {
			if _, ok := templates[child.Extends]; !ok {
				return rawYAMLTemplate{}, fmt.Errorf("template %q extends unknown template %q", name, child.Extends)
			}
			parent, err := resolve(child.Extends)
			if err != nil {
				return rawYAMLTemplate{}, err
			}
			child = mergeTemplate(parent, child)
		}
		path = path[:len(path)-1]
		states[name] = templateResolved
		resolved[name] = child
		return child, nil
	}
	for _, name := range sortedMapKeys(templates) {
		if _, err := resolve(name); err != nil {
			return nil, err
		}
	}
	return resolved, nil
}

func mergeTemplate(parent rawYAMLTemplate, child rawYAMLTemplate) rawYAMLTemplate {
	merged := parent
	merged.Extends = child.Extends
	merged.Abstract = child.Abstract
	merged.Repository = mergeRepository(parent.Repository, child.Repository)
	if child.Containers != nil {
		merged.Containers = child.Containers
	}
	if child.Session != nil {
		merged.Session = child.Session
	}
	return merged
}

func mergeRepository(parent *rawRepositoryTemplate, child *rawRepositoryTemplate) *rawRepositoryTemplate {
	if child == nil {
		return parent
	}
	merged := rawRepositoryTemplate{}
	if parent != nil {
		merged = *parent
	}
	if child.Path != nil {
		merged.Path = child.Path
	}
	if child.Base != nil {
		merged.Base = child.Base
	}
	if child.Prefix != nil {
		merged.Prefix = child.Prefix
	}
	return &merged
}

func (raw rawYAMLTemplate) toDomain(name string, vars *domain.TemplateVars) (domain.Template, error) {
	sources := []domain.SourceTemplate{}
	if raw.Repository != nil {
		source, err := domain.NewSourceTemplate(stringValue(raw.Repository.Path), stringValue(raw.Repository.Base), stringValue(raw.Repository.Prefix))
		if err != nil {
			return domain.Template{}, fmt.Errorf("template %q: %w", name, err)
		}
		sources = append(sources, source)
	}
	containers := []domain.ContainerTemplate{}
	if raw.Containers != nil && raw.Containers.Services != nil {
		for _, containerName := range sortedMapKeys(*raw.Containers.Services) {
			rawContainer := (*raw.Containers.Services)[containerName]
			modeValue := ""
			if rawContainer.Mode != nil {
				modeValue = *rawContainer.Mode
			}
			mode, err := domain.NewMode(modeValue)
			if err != nil {
				return domain.Template{}, fmt.Errorf("container %q: %w", containerName, err)
			}
			environments := make([]domain.Environment, 0, len(rawContainer.Environment))
			for _, environmentName := range sortedMapKeys(rawContainer.Environment) {
				value, err := renderValue(rawContainer.Environment[environmentName], vars)
				if err != nil {
					return domain.Template{}, fmt.Errorf("render environment %q for container %q: %w", environmentName, containerName, err)
				}
				environment, err := domain.NewEnvironment(environmentName, value)
				if err != nil {
					return domain.Template{}, err
				}
				environments = append(environments, environment)
			}
			mounts := make([]domain.Mount, 0, len(rawContainer.Files))
			for _, target := range sortedMapKeys(rawContainer.Files) {
				mount, err := domain.NewMount(target, rawContainer.Files[target])
				if err != nil {
					return domain.Template{}, fmt.Errorf("container %q: %w", containerName, err)
				}
				mounts = append(mounts, mount)
			}
			container, err := domain.NewContainerTemplate(containerName, mode, environments, mounts)
			if err != nil {
				return domain.Template{}, err
			}
			containers = append(containers, container)
		}
	}
	windows := []domain.Window{}
	if raw.Session != nil && raw.Session.Windows != nil {
		for _, rawWindow := range *raw.Session.Windows {
			command, err := renderValue(rawWindow.Command, vars)
			if err != nil {
				return domain.Template{}, fmt.Errorf("render session window %q: %w", rawWindow.Name, err)
			}
			window, err := domain.NewWindow(rawWindow.Name, command)
			if err != nil {
				return domain.Template{}, err
			}
			windows = append(windows, window)
		}
	}
	return domain.NewTemplate(name, sources, containers, domain.NewSessionTemplate(windows))
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func sortedMapKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func renderValue(value string, vars *domain.TemplateVars) (string, error) {
	if vars == nil {
		return value, nil
	}
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

func (a YAMLConfigAdapter) ConfigExists(ctx context.Context) (bool, error) {
	_ = ctx
	_, err := os.Stat(a.Path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func (a YAMLConfigAdapter) SaveConfig(ctx context.Context, cfg domain.Templates) error {
	_ = ctx
	if err := os.MkdirAll(filepath.Join(filepath.Dir(a.Path), ".paracell"), 0o755); err != nil {
		return err
	}
	raw := yamlConfig{
		Providers: yamlProviders{
			Source: string(cfg.SourceDriverType), Container: string(cfg.ContainerDriverType),
			Session: string(cfg.SessionDriverType), Notifications: string(cfg.NotificationDriverType),
		},
		Templates: make(map[string]rawYAMLTemplate, len(cfg.Templates)),
	}
	raw.Project.Name = cfg.ProjectName
	for _, item := range cfg.Templates {
		entry := rawYAMLTemplate{}
		if len(item.Sources) > 0 {
			source := item.Sources[0]
			entry.Repository = &rawRepositoryTemplate{Path: stringPointer(source.Path), Base: stringPointer(source.Base), Prefix: stringPointer(source.Prefix)}
		}
		services := make(map[string]rawContainer, len(item.Containers))
		for _, container := range item.Containers {
			mode := string(container.Mode)
			environment := make(map[string]string, len(container.Environments))
			for _, item := range container.Environments {
				environment[item.Name] = item.Value
			}
			files := make(map[string]string, len(container.Mounts))
			for _, mount := range container.Mounts {
				files[mount.TargetPath] = mount.SourcePath
			}
			services[container.Name] = rawContainer{Mode: &mode, Environment: environment, Files: files}
		}
		entry.Containers = &rawContainerTemplate{Services: &services}
		windows := make([]rawWindow, 0, len(item.Session.Windows))
		for _, window := range item.Session.Windows {
			windows = append(windows, rawWindow{Name: window.Name, Command: window.Command})
		}
		entry.Session = &rawSessionTemplate{Windows: &windows}
		raw.Templates[item.Name] = entry
	}
	data, err := yaml.Marshal(raw)
	if err != nil {
		return err
	}
	return os.WriteFile(a.Path, data, 0o644)
}

func stringPointer(value string) *string {
	return &value
}
