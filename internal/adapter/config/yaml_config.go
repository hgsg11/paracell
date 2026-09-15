package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/hgsg11/paracell/internal/domain"
	"gopkg.in/yaml.v3"
)

type YAMLConfigAdapter struct {
	Path string
}

func NewYAMLConfigAdapter(path string) YAMLConfigAdapter { return YAMLConfigAdapter{Path: path} }

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

func (a YAMLConfigAdapter) Load(ctx context.Context) (domain.Templates, error) {
	_ = ctx
	data, err := os.ReadFile(a.Path)
	if err != nil {
		return domain.Templates{}, err
	}
	var raw yamlConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
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
	items := make([]domain.Template, 0, len(raw.Templates))
	for _, name := range sortedMapKeys(raw.Templates) {
		domainTemplate, err := raw.Templates[name].toDomain(name)
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

func (raw rawYAMLTemplate) toDomain(name string) (domain.Template, error) {
	var repository *domain.SourceTemplate
	if raw.Repository != nil {
		source, err := domain.NewPartialSourceTemplate(raw.Repository.Path, raw.Repository.Base, raw.Repository.Prefix)
		if err != nil {
			return domain.Template{}, fmt.Errorf("template %q: %w", name, err)
		}
		repository = &source
	}
	var containers *[]domain.ContainerTemplate
	if raw.Containers != nil && raw.Containers.Services != nil {
		items := []domain.ContainerTemplate{}
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
				environment, err := domain.NewEnvironment(environmentName, rawContainer.Environment[environmentName])
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
			items = append(items, container)
		}
		containers = &items
	} else if raw.Containers != nil {
		empty := []domain.ContainerTemplate{}
		containers = &empty
	}
	var session *domain.SessionTemplate
	if raw.Session != nil && raw.Session.Windows != nil {
		windows := []domain.Window{}
		for _, rawWindow := range *raw.Session.Windows {
			window, err := domain.NewWindow(rawWindow.Name, rawWindow.Command)
			if err != nil {
				return domain.Template{}, err
			}
			windows = append(windows, window)
		}
		value := domain.NewSessionTemplate(windows)
		session = &value
	} else if raw.Session != nil {
		value := domain.NewSessionTemplate(nil)
		session = &value
	}
	return domain.NewUnresolvedTemplate(name, raw.Extends, raw.Abstract, repository, containers, session)
}

func sortedMapKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
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
		entry := rawYAMLTemplate{Extends: item.Extends, Abstract: item.Abstract}
		if item.Repository != nil {
			source := *item.Repository
			entry.Repository = &rawRepositoryTemplate{Path: stringPointer(source.Path), Base: stringPointer(source.Base), Prefix: stringPointer(source.Prefix)}
		}
		if item.Containers != nil {
			services := make(map[string]rawContainer, len(*item.Containers))
			for _, container := range *item.Containers {
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
		}
		if item.Session != nil {
			windows := make([]rawWindow, 0, len(item.Session.Windows))
			for _, window := range item.Session.Windows {
				windows = append(windows, rawWindow{Name: window.Name, Command: window.Command})
			}
			entry.Session = &rawSessionTemplate{Windows: &windows}
		}
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
