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
	Workspace     string `yaml:"workspace"`
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
	Extends   string                `yaml:"extends,omitempty"`
	Abstract  bool                  `yaml:"abstract,omitempty"`
	Commander *rawCommanderCellSpec `yaml:"commanderCell,omitempty"`
}

type rawCommanderCellSpec struct {
	Name         string                           `yaml:"name"`
	Workspace    *rawWorkspaceTemplate            `yaml:"workspace,omitempty"`
	Targets      map[string]rawTargetCellSpec     `yaml:"targets,omitempty"`
	Dependencies map[string]rawDependencyCellSpec `yaml:"dependencies,omitempty"`
}

type rawTargetCellSpec struct {
	Source       *rawRepositoryTemplate `yaml:"source,omitempty"`
	Container    *rawContainer          `yaml:"container,omitempty"`
	Dependencies []string               `yaml:"dependencies,omitempty"`
}

type rawDependencyCellSpec struct {
	Container *rawContainer `yaml:"container"`
}

type rawRepositoryTemplate struct {
	Path   *string `yaml:"path,omitempty"`
	Base   *string `yaml:"base,omitempty"`
	Prefix *string `yaml:"branchPrefix,omitempty"`
}

type rawContainer struct {
	Environment map[string]string `yaml:"environment,omitempty"`
	Files       map[string]string `yaml:"files,omitempty"`
}

type rawWorkspaceTemplate struct {
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
	workspaceDriver, err := domain.NewWorkspaceDriverType(raw.Providers.Workspace)
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
	templates, err := domain.NewTemplates(raw.Project.Name, items, workspaceDriver, domain.NewContainerDriverType(raw.Providers.Container), sourceDriver, notificationDriver)
	if err != nil {
		return domain.Templates{}, err
	}
	return templates, nil
}

func (raw rawYAMLTemplate) toDomain(name string) (domain.Template, error) {
	var commander *domain.CommanderCellSpec
	if raw.Commander != nil {
		parsed, err := raw.Commander.toDomain()
		if err != nil {
			return domain.Template{}, fmt.Errorf("template %q: %w", name, err)
		}
		commander = &parsed
	}
	return domain.NewUnresolvedTemplate(name, raw.Extends, raw.Abstract, commander)
}

func (raw rawCommanderCellSpec) toDomain() (domain.CommanderCellSpec, error) {
	if raw.Workspace == nil {
		return domain.CommanderCellSpec{}, fmt.Errorf("commanderCell.workspace is required")
	}
	windows := []domain.Window{}
	if raw.Workspace.Windows != nil {
		for _, rawWindow := range *raw.Workspace.Windows {
			window, err := domain.NewWindow(rawWindow.Name, rawWindow.Command)
			if err != nil {
				return domain.CommanderCellSpec{}, err
			}
			windows = append(windows, window)
		}
	}
	targets := make([]domain.TargetCellSpec, 0, len(raw.Targets))
	for _, name := range sortedMapKeys(raw.Targets) {
		value := raw.Targets[name]
		var source *domain.SourceTemplate
		if value.Source != nil {
			parsed, err := domain.NewPartialSourceTemplate(value.Source.Path, value.Source.Base, value.Source.Prefix)
			if err != nil {
				return domain.CommanderCellSpec{}, fmt.Errorf("target cell %q source: %w", name, err)
			}
			source = &parsed
		}
		var container *domain.ContainerTemplate
		if value.Container != nil {
			parsed, err := containerTemplate(name, domain.Target, *value.Container)
			if err != nil {
				return domain.CommanderCellSpec{}, fmt.Errorf("target cell %q: %w", name, err)
			}
			container = &parsed
		}
		target, err := domain.NewTargetCellSpec(name, source, container, value.Dependencies)
		if err != nil {
			return domain.CommanderCellSpec{}, err
		}
		targets = append(targets, target)
	}
	dependencies := make([]domain.DependencyCellSpec, 0, len(raw.Dependencies))
	for _, name := range sortedMapKeys(raw.Dependencies) {
		value := raw.Dependencies[name]
		if value.Container == nil {
			return domain.CommanderCellSpec{}, fmt.Errorf("dependency cell %q requires a container", name)
		}
		container, err := containerTemplate(name, domain.Dependency, *value.Container)
		if err != nil {
			return domain.CommanderCellSpec{}, fmt.Errorf("dependency cell %q: %w", name, err)
		}
		dependency, err := domain.NewDependencyCellSpec(name, container)
		if err != nil {
			return domain.CommanderCellSpec{}, err
		}
		dependencies = append(dependencies, dependency)
	}
	return domain.NewCommanderCellSpec(raw.Name, domain.NewWorkspaceTemplate(windows), targets, dependencies)
}

func containerTemplate(name string, mode domain.Mode, raw rawContainer) (domain.ContainerTemplate, error) {
	environments := make([]domain.Environment, 0, len(raw.Environment))
	for _, environmentName := range sortedMapKeys(raw.Environment) {
		environment, err := domain.NewEnvironment(environmentName, raw.Environment[environmentName])
		if err != nil {
			return domain.ContainerTemplate{}, err
		}
		environments = append(environments, environment)
	}
	mounts := make([]domain.Mount, 0, len(raw.Files))
	for _, target := range sortedMapKeys(raw.Files) {
		mount, err := domain.NewMount(target, raw.Files[target])
		if err != nil {
			return domain.ContainerTemplate{}, fmt.Errorf("container %q: %w", name, err)
		}
		mounts = append(mounts, mount)
	}
	return domain.NewContainerTemplate(name, mode, environments, mounts)
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
			Workspace: string(cfg.WorkspaceDriverType), Notifications: string(cfg.NotificationDriverType),
		},
		Templates: make(map[string]rawYAMLTemplate, len(cfg.Templates)),
	}
	raw.Project.Name = cfg.ProjectName
	for _, item := range cfg.Templates {
		entry := rawYAMLTemplate{Extends: item.Extends, Abstract: item.Abstract}
		if item.Commander != nil {
			commander := item.Commander
			workspaceWindows := make([]rawWindow, 0, len(commander.Workspace.Windows))
			for _, window := range commander.Workspace.Windows {
				workspaceWindows = append(workspaceWindows, rawWindow{Name: window.Name, Command: window.Command})
			}
			rawCommander := &rawCommanderCellSpec{
				Name:         commander.Name,
				Workspace:    &rawWorkspaceTemplate{Windows: &workspaceWindows},
				Targets:      make(map[string]rawTargetCellSpec, len(commander.Targets)),
				Dependencies: make(map[string]rawDependencyCellSpec, len(commander.Dependencies)),
			}
			for _, target := range commander.Targets {
				rawTarget := rawTargetCellSpec{Dependencies: append([]string(nil), target.Dependencies...)}
				if target.Source != nil {
					source := *target.Source
					rawTarget.Source = &rawRepositoryTemplate{Path: stringPointer(source.Path), Base: stringPointer(source.Base), Prefix: stringPointer(source.Prefix)}
				}
				if target.Container != nil {
					rawTarget.Container = rawContainerFromTemplate(*target.Container)
				}
				rawCommander.Targets[target.Name] = rawTarget
			}
			for _, dependency := range commander.Dependencies {
				rawCommander.Dependencies[dependency.Name] = rawDependencyCellSpec{Container: rawContainerFromTemplate(dependency.Container)}
			}
			entry.Commander = rawCommander
		}
		raw.Templates[item.Name] = entry
	}
	data, err := yaml.Marshal(raw)
	if err != nil {
		return err
	}
	return os.WriteFile(a.Path, data, 0o644)
}

func rawContainerFromTemplate(container domain.ContainerTemplate) *rawContainer {
	environment := make(map[string]string, len(container.Environments))
	for _, item := range container.Environments {
		environment[item.Name] = item.Value
	}
	files := make(map[string]string, len(container.Mounts))
	for _, mount := range container.Mounts {
		files[mount.TargetPath] = mount.SourcePath
	}
	return &rawContainer{Environment: environment, Files: files}
}

func stringPointer(value string) *string {
	return &value
}
