package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/shige1114/paradev/internal/domain"
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
	Source    string `yaml:"source"`
	Container string `yaml:"container"`
	Session   string `yaml:"session"`
}

type yamlTemplate struct {
	Repository domain.RepositoryTemplate `yaml:"repository"`
	Containers domain.ContainerTemplate  `yaml:"containers"`
	Session    domain.SessionTemplate    `yaml:"session"`
}

func (a YAMLConfigAdapter) Load(ctx context.Context) (domain.Config, error) {
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
	for name, template := range raw.Templates {
		templates[name] = domain.Template{
			Name:       name,
			Repository: template.Repository,
			Containers: template.Containers,
			Session:    template.Session,
		}
	}
	providers := domain.ProviderConfig{
		Source:    raw.Providers.Source,
		Container: raw.Providers.Container,
		Session:   raw.Providers.Session,
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

func validateProviders(providers domain.ProviderConfig) error {
	if providers.Source == "" {
		return errors.New("providers.source is required")
	}
	if providers.Source != "git" {
		return fmt.Errorf("unsupported providers.source %q", providers.Source)
	}
	if providers.Container == "" {
		return errors.New("providers.container is required")
	}
	if providers.Container != "docker" {
		return fmt.Errorf("unsupported providers.container %q", providers.Container)
	}
	if providers.Session == "" {
		return errors.New("providers.session is required")
	}
	if providers.Session != "tmux" {
		return fmt.Errorf("unsupported providers.session %q", providers.Session)
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
	if err := os.MkdirAll(filepath.Join(filepath.Dir(a.Path), ".pdev"), 0o755); err != nil {
		return err
	}
	raw := yamlConfig{
		Providers: yamlProviders{
			Source:    cfg.Providers.Source,
			Container: cfg.Providers.Container,
			Session:   cfg.Providers.Session,
		},
		Templates: make(map[string]yamlTemplate, len(cfg.Templates)),
	}
	raw.Project.Name = cfg.Project.Name
	for name, template := range cfg.Templates {
		raw.Templates[name] = yamlTemplate{
			Repository: template.Repository,
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
