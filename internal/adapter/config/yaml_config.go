package config

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

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
	Files      []string                  `yaml:"files,omitempty"`
	Containers domain.ContainerTemplate  `yaml:"containers"`
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
	for name, rawTemplate := range raw.Templates {
		rendered, err := instantiateTemplate(domain.Template{
			Repository: rawTemplate.Repository,
			Files:      append([]string(nil), rawTemplate.Files...),
			Containers: rawTemplate.Containers,
			Session:    rawTemplate.Session,
		}, vars)
		if err != nil {
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

func instantiateTemplate(tpl domain.Template, vars *domain.TemplateVars) (domain.Template, error) {
	if vars == nil {
		return tpl, nil
	}
	rendered := tpl
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
	tmpl, err := template.New("value").Option("missingkey=error").Parse(value)
	if err != nil {
		return "", err
	}
	var b bytes.Buffer
	if err := tmpl.Execute(&b, map[string]string{
		"issue": vars.Issue,
		"name":  vars.Name,
	}); err != nil {
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
