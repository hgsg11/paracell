package config

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

func (a YAMLConfigAdapter) Load(ctx context.Context, vars *domain.TemplateVars) (domain.Config, error) {
	_ = ctx
	data, err := os.ReadFile(a.Path)
	if err != nil {
		return domain.Config{}, err
	}
	var raw yamlConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return domain.Config{}, err
	}
	templates := make(map[string]domain.Template, len(raw.Templates))
	templateVars := vars
	if vars != nil {
		copied := *vars
		copied.Project = raw.Project.Name
		templateVars = &copied
	}
	for name, rawTemplate := range raw.Templates {
		rendered, err := instantiateTemplate(domain.Template{
			Repository: rawTemplate.Repository,
			Files:      append([]string(nil), rawTemplate.Files...),
			Containers: rawTemplate.Containers,
			Session:    rawTemplate.Session,
		}, templateVars)
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
		Project:   domain.ProjectConfig{Name: raw.Project.Name},
		Providers: providers,
		Templates: templates,
	}, nil
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
	for role, service := range services {
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
	for role, service := range tpl.Containers.Services {
		renderedService := service
		if service.Environment != nil {
			renderedService.Environment = make(map[string]string, len(service.Environment))
			for name, value := range service.Environment {
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
