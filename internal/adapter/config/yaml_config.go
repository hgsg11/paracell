package config

import (
	"context"
	"os"

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
	Templates map[string]yamlTemplate `yaml:"templates"`
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
	return domain.Config{
		Project:   domain.ProjectConfig{Name: raw.Project.Name},
		Templates: templates,
	}, nil
}
