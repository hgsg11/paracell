package config

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/hgsg11/paracell/internal/domain"
	"gopkg.in/yaml.v3"
)

type YAMLConfigAdapter struct {
	Path string
}

type yamlConfig struct {
	Project struct {
		Name string `yaml:"name"`
	} `yaml:"project"`
	Providers yamlProviders           `yaml:"providers"`
	Templates map[string]yamlTemplate `yaml:"templates"`
}

type yamlProviders struct {
	Source        string `yaml:"source"`
	Container     string `yaml:"container,omitempty"`
	Session       string `yaml:"session"`
	Notifications string `yaml:"notifications"`
}

type yamlTemplate struct {
	Repository domain.RepositoryTemplate `yaml:"repository"`
	Files      []string                  `yaml:"files,omitempty"`
	Containers domain.ContainerTemplate  `yaml:"containers,omitempty"`
	Session    domain.SessionTemplate    `yaml:"session"`
}

// yamlLoadConfig keeps field presence while decoding so inheritance can
// distinguish an omitted value from an explicitly empty value. yamlConfig is
// intentionally retained for SaveConfig's backwards-compatible output.
type yamlLoadConfig struct {
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
	Files      *[]string              `yaml:"files,omitempty"`
	Containers *rawContainerTemplate  `yaml:"containers,omitempty"`
	Session    *rawSessionTemplate    `yaml:"session,omitempty"`
}

type rawRepositoryTemplate struct {
	BranchPrefix *string `yaml:"branchPrefix,omitempty"`
	Base         *string `yaml:"base,omitempty"`
	BranchMode   *string `yaml:"branchMode,omitempty"`
}

type rawContainerTemplate struct {
	Network  *string                                     `yaml:"network,omitempty"`
	Services *map[string]domain.ContainerServiceTemplate `yaml:"services,omitempty"`
}

type rawSessionTemplate struct {
	Windows *[]domain.SessionWindowTemplate `yaml:"windows,omitempty"`
}

func (a YAMLConfigAdapter) Load(ctx context.Context, vars *domain.TemplateVars) (domain.Config, error) {
	_ = ctx
	data, err := os.ReadFile(a.Path)
	if err != nil {
		return domain.Config{}, err
	}
	var raw yamlLoadConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return domain.Config{}, err
	}
	resolvedTemplates, err := resolveTemplates(raw.Templates)
	if err != nil {
		return domain.Config{}, err
	}
	templates := make(map[string]domain.Template, len(resolvedTemplates))
	abstractTemplates := make(map[string]struct{})
	templateVars := vars
	if vars != nil {
		copied := *vars
		copied.Project = raw.Project.Name
		templateVars = &copied
	}
	names := sortedTemplateNames(resolvedTemplates)
	for _, name := range names {
		rawTemplate := resolvedTemplates[name]
		if rawTemplate.Abstract {
			abstractTemplates[name] = struct{}{}
			continue
		}
		rendered, err := instantiateTemplate(rawTemplate.domainTemplate(name), templateVars)
		if err != nil {
			return domain.Config{}, err
		}
		if err := validateRepositoryBranchMode(name, rendered.Repository); err != nil {
			return domain.Config{}, err
		}
		if err := validateContainerTemplate(rendered.Containers); err != nil {
			return domain.Config{}, err
		}
		templates[name] = domain.Template{
			Name:       name,
			Repository: rendered.Repository,
			Files:      append([]string(nil), rendered.Files...),
			Containers: rendered.Containers,
			Session:    rendered.Session,
		}
	}
	providers := domain.ProviderConfig{
		Source:        raw.Providers.Source,
		Container:     raw.Providers.Container,
		Session:       raw.Providers.Session,
		Notifications: raw.Providers.Notifications,
	}
	if err := validateProviders(providers); err != nil {
		return domain.Config{}, err
	}
	return domain.Config{
		Project:           domain.ProjectConfig{Name: raw.Project.Name},
		Providers:         providers,
		Templates:         templates,
		AbstractTemplates: abstractTemplates,
	}, nil
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
			cycleStart := 0
			for i, item := range path {
				if item == name {
					cycleStart = i
					break
				}
			}
			cycle := append(append([]string(nil), path[cycleStart:]...), name)
			quoted := make([]string, len(cycle))
			for i, item := range cycle {
				quoted[i] = fmt.Sprintf("%q", item)
			}
			return rawYAMLTemplate{}, fmt.Errorf("template inheritance cycle: %s", strings.Join(quoted, " -> "))
		}

		child := templates[name]
		states[name] = templateVisiting
		path = append(path, name)
		defer func() { path = path[:len(path)-1] }()

		merged := child
		if child.Extends != "" {
			if _, ok := templates[child.Extends]; !ok {
				return rawYAMLTemplate{}, fmt.Errorf("template %q extends unknown template %q", name, child.Extends)
			}
			parent, err := resolve(child.Extends)
			if err != nil {
				return rawYAMLTemplate{}, err
			}
			merged = mergeRawTemplate(parent, child)
		}
		states[name] = templateResolved
		resolved[name] = merged
		return merged, nil
	}

	for _, name := range sortedTemplateNames(templates) {
		if _, err := resolve(name); err != nil {
			return nil, err
		}
	}
	return resolved, nil
}

func mergeRawTemplate(parent, child rawYAMLTemplate) rawYAMLTemplate {
	merged := parent
	merged.Extends = child.Extends
	// Abstract is a property of the declared template, not an inherited field.
	merged.Abstract = child.Abstract
	merged.Repository = mergeRawRepository(parent.Repository, child.Repository)
	if child.Files != nil {
		merged.Files = child.Files
	}
	merged.Containers = mergeRawContainers(parent.Containers, child.Containers)
	merged.Session = mergeRawSession(parent.Session, child.Session)
	return merged
}

func mergeRawRepository(parent, child *rawRepositoryTemplate) *rawRepositoryTemplate {
	if child == nil {
		return parent
	}
	merged := rawRepositoryTemplate{}
	if parent != nil {
		merged = *parent
	}
	if child.BranchPrefix != nil {
		merged.BranchPrefix = child.BranchPrefix
	}
	if child.Base != nil {
		merged.Base = child.Base
	}
	if child.BranchMode != nil {
		merged.BranchMode = child.BranchMode
	}
	return &merged
}

func mergeRawContainers(parent, child *rawContainerTemplate) *rawContainerTemplate {
	if child == nil {
		return parent
	}
	merged := rawContainerTemplate{}
	if parent != nil {
		merged = *parent
	}
	if child.Network != nil {
		merged.Network = child.Network
	}
	if child.Services != nil {
		merged.Services = child.Services
	}
	return &merged
}

func mergeRawSession(parent, child *rawSessionTemplate) *rawSessionTemplate {
	if child == nil {
		return parent
	}
	merged := rawSessionTemplate{}
	if parent != nil {
		merged = *parent
	}
	if child.Windows != nil {
		merged.Windows = child.Windows
	}
	return &merged
}

func (raw rawYAMLTemplate) domainTemplate(name string) domain.Template {
	tpl := domain.Template{Name: name}
	if raw.Repository != nil {
		tpl.Repository = domain.RepositoryTemplate{
			BranchPrefix: stringValue(raw.Repository.BranchPrefix),
			Base:         stringValue(raw.Repository.Base),
			BranchMode:   stringValue(raw.Repository.BranchMode),
		}
	}
	if raw.Files != nil {
		tpl.Files = append([]string(nil), (*raw.Files)...)
	}
	if raw.Containers != nil {
		tpl.Containers.Network = domain.ContainerNetwork(stringValue(raw.Containers.Network))
		if raw.Containers.Services != nil {
			tpl.Containers.Services = cloneServices(*raw.Containers.Services)
		}
	}
	if raw.Session != nil && raw.Session.Windows != nil {
		tpl.Session.Windows = append([]domain.SessionWindowTemplate(nil), (*raw.Session.Windows)...)
	}
	return tpl
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func cloneServices(services map[string]domain.ContainerServiceTemplate) map[string]domain.ContainerServiceTemplate {
	cloned := make(map[string]domain.ContainerServiceTemplate, len(services))
	for name, service := range services {
		copy := service
		if service.Environment != nil {
			copy.Environment = make(map[string]string, len(service.Environment))
			for key, value := range service.Environment {
				copy.Environment[key] = value
			}
		}
		if service.Database != nil {
			database := *service.Database
			database.InitFiles = append([]string(nil), service.Database.InitFiles...)
			copy.Database = &database
		}
		cloned[name] = copy
	}
	return cloned
}

func sortedTemplateNames[T any](templates map[string]T) []string {
	return sortedMapKeys(templates)
}

func sortedMapKeys[T any](values map[string]T) []string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func validateRepositoryBranchMode(name string, repository domain.RepositoryTemplate) error {
	switch repository.BranchMode {
	case "", domain.RepositoryBranchModeCreate, domain.RepositoryBranchModeReuse, domain.RepositoryBranchModeRequire:
		return nil
	default:
		return fmt.Errorf("unsupported repository.branchMode %q for template %q", repository.BranchMode, name)
	}
}

func validateContainerTemplate(containers domain.ContainerTemplate) error {
	switch containers.Network {
	case "", domain.ContainerNetworkIsolated, domain.ContainerNetworkShared:
	default:
		return fmt.Errorf("unsupported containers.network %q", containers.Network)
	}
	return validateContainerServices(containers.Services)
}

func validateContainerServices(services map[string]domain.ContainerServiceTemplate) error {
	for _, role := range sortedMapKeys(services) {
		service := services[role]
		switch service.VolumeMode {
		case "", "readonly", "copy":
		default:
			return fmt.Errorf("unsupported volumeMode %q for service %q", service.VolumeMode, role)
		}
		if service.Database == nil {
			if role == "db" && service.VolumeMode != "" {
				return fmt.Errorf("volumeMode is not supported for service %q", role)
			}
			continue
		}
		if role != "db" {
			return fmt.Errorf("database config is only supported for service %q", "db")
		}
		if service.VolumeMode != "copy" {
			return fmt.Errorf("database service %q requires volumeMode %q", role, "copy")
		}
		switch service.Database.System {
		case "mysql":
		default:
			return fmt.Errorf("unsupported databaseSystem %q for service %q", service.Database.System, role)
		}
		switch service.Database.CopyMode {
		case "", "schema", "data":
		default:
			return fmt.Errorf("unsupported copyMode %q for service %q", service.Database.CopyMode, role)
		}
		for _, file := range service.Database.InitFiles {
			if filepath.IsAbs(file) {
				return fmt.Errorf("initFiles path %q for service %q must be relative", file, role)
			}
			clean := filepath.Clean(file)
			if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
				return fmt.Errorf("initFiles path %q for service %q must stay within project root", file, role)
			}
		}
	}
	return nil
}

func instantiateTemplate(tpl domain.Template, vars *domain.TemplateVars) (domain.Template, error) {
	if vars == nil {
		return tpl, nil
	}
	rendered := tpl
	rendered.Containers.Services = make(map[string]domain.ContainerServiceTemplate, len(tpl.Containers.Services))
	for _, role := range sortedMapKeys(tpl.Containers.Services) {
		service := tpl.Containers.Services[role]
		renderedService := service
		if service.Environment != nil {
			renderedService.Environment = make(map[string]string, len(service.Environment))
			for _, name := range sortedMapKeys(service.Environment) {
				value := service.Environment[name]
				renderedValue, err := renderEnvironmentTemplate(value, vars)
				if err != nil {
					return domain.Template{}, fmt.Errorf("render environment %q for service %q: %w", name, role, err)
				}
				renderedService.Environment[name] = renderedValue
			}
		}
		rendered.Containers.Services[role] = renderedService
	}
	rendered.Session.Windows = make([]domain.SessionWindowTemplate, 0, len(tpl.Session.Windows))
	for _, window := range tpl.Session.Windows {
		command, err := renderTemplate(window.Command, vars)
		if err != nil {
			return domain.Template{}, err
		}
		rendered.Session.Windows = append(rendered.Session.Windows, domain.SessionWindowTemplate{
			Name:    window.Name,
			Command: command,
		})
	}
	return rendered, nil
}

func renderTemplate(value string, vars *domain.TemplateVars) (string, error) {
	return renderValue(value, map[string]string{
		"issue":   vars.Issue,
		"name":    vars.Name,
		"Command": vars.Command,
	})
}

func renderEnvironmentTemplate(value string, vars *domain.TemplateVars) (string, error) {
	return renderValue(value, map[string]string{
		"issue":   vars.Issue,
		"name":    vars.Name,
		"project": vars.Project,
	})
}

func renderValue(value string, vars map[string]string) (string, error) {
	tmpl, err := template.New("value").Option("missingkey=error").Parse(value)
	if err != nil {
		return "", err
	}
	var b bytes.Buffer
	if err := tmpl.Execute(&b, vars); err != nil {
		return "", err
	}
	return b.String(), nil
}

func validateProviders(providers domain.ProviderConfig) error {
	if providers.Source == "" {
		return errors.New("providers.source is required")
	}
	if providers.Source != "git" {
		return fmt.Errorf("unsupported providers.source %q", providers.Source)
	}
	if providers.Container != "" && providers.Container != "docker" {
		return fmt.Errorf("unsupported providers.container %q", providers.Container)
	}
	if providers.Session == "" {
		return errors.New("providers.session is required")
	}
	if providers.Session != "tmux" {
		return fmt.Errorf("unsupported providers.session %q", providers.Session)
	}
	if providers.Notifications != "" && providers.Notifications != "tmux" {
		return fmt.Errorf("unsupported providers.notifications %q", providers.Notifications)
	}
	return nil
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

func (a YAMLConfigAdapter) SaveConfig(ctx context.Context, cfg domain.Config) error {
	_ = ctx
	if err := os.MkdirAll(filepath.Join(filepath.Dir(a.Path), ".paracell"), 0o755); err != nil {
		return err
	}
	raw := yamlConfig{
		Providers: yamlProviders{
			Source:        cfg.Providers.Source,
			Container:     cfg.Providers.Container,
			Session:       cfg.Providers.Session,
			Notifications: cfg.Providers.Notifications,
		},
		Templates: make(map[string]yamlTemplate, len(cfg.Templates)),
	}
	raw.Project.Name = cfg.Project.Name
	for name, template := range cfg.Templates {
		raw.Templates[name] = yamlTemplate{
			Repository: template.Repository,
			Files:      append([]string(nil), template.Files...),
			Containers: template.Containers,
			Session:    template.Session,
		}
	}
	data, err := yaml.Marshal(raw)
	if err != nil {
		return err
	}
	return os.WriteFile(a.Path, data, 0o644)
}
