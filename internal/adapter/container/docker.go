package container

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"

	"github.com/shige1114/paradev/internal/adapter/system"
	"github.com/shige1114/paradev/internal/domain"
)

type RunSpec struct {
	Name       string
	Image      string
	Network    string
	Env        []string
	Entrypoint []string
	Command    []string
	WorkDir    string
	Mounts     []string
	Ports      map[string]string
}

func BuildDockerRunArgs(spec RunSpec) []string {
	args := []string{"run", "-d", "--name", spec.Name}
	if spec.Network != "" {
		args = append(args, "--network", spec.Network)
	}
	for _, env := range spec.Env {
		args = append(args, "-e", env)
	}
	if len(spec.Entrypoint) > 0 {
		args = append(args, "--entrypoint", spec.Entrypoint[0])
	}
	if spec.WorkDir != "" {
		args = append(args, "-w", spec.WorkDir)
	}
	for _, mount := range spec.Mounts {
		args = append(args, "-v", mount)
	}
	hostPorts := make([]string, 0, len(spec.Ports))
	for host := range spec.Ports {
		hostPorts = append(hostPorts, host)
	}
	sort.Strings(hostPorts)
	for _, host := range hostPorts {
		args = append(args, "-p", host+":"+spec.Ports[host])
	}
	args = append(args, spec.Image)
	args = append(args, spec.Command...)
	return args
}

type DockerCLIAdapter struct {
	Runner system.Runner
	Root   string
}

func (a DockerCLIAdapter) CreateContainers(ctx context.Context, cell domain.Cell, template domain.Template) error {
	for role, service := range cell.Containers.Services {
		source := template.Containers.Services[role].SourceContainer
		if source == "" {
			source = service.SourceContainer
		}
		inspection, err := a.inspectContainer(ctx, source)
		if err != nil {
			return err
		}
		args := BuildDockerRunArgs(RunSpec{
			Name:    service.ContainerName,
			Image:   inspection.Config.Image,
			Network: firstNetwork(inspection.NetworkSettings.Networks),
			Env:     append([]string(nil), inspection.Config.Env...),
			Mounts:  a.cellMounts(cell, inspection.Mounts),
		})
		if err := a.Runner.Run(ctx, "docker", args...); err != nil {
			return err
		}
	}
	return nil
}

func (a DockerCLIAdapter) inspectContainer(ctx context.Context, source string) (containerInspection, error) {
	raw, err := a.Runner.Output(ctx, "docker", "inspect", "-f", "{{json .}}", source)
	if err != nil {
		return containerInspection{}, err
	}
	var inspection containerInspection
	if err := json.Unmarshal([]byte(raw), &inspection); err != nil {
		return containerInspection{}, err
	}
	return inspection, nil
}

func (a DockerCLIAdapter) cellMounts(cell domain.Cell, mounts []dockerMount) []string {
	out := make([]string, 0, len(mounts))
	for _, mount := range mounts {
		if mount.Type == "volume" && mount.Name != "" {
			out = append(out, mount.Name+":"+mount.Destination+":ro")
			continue
		}
		if mount.Type != "bind" {
			continue
		}
		rel, err := filepath.Rel(a.Root, mount.Source)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		source := cell.Source.Path
		if rel != "." {
			source = filepath.Join(cell.Source.Path, rel)
		}
		spec := source + ":" + mount.Destination
		if !mount.RW {
			spec += ":ro"
		}
		out = append(out, spec)
	}
	return out
}

func firstNetwork(networks map[string]dockerNetwork) string {
	names := make([]string, 0, len(networks))
	for name := range networks {
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) == 0 {
		return ""
	}
	return names[0]
}

func (a DockerCLIAdapter) RemoveContainers(ctx context.Context, cell domain.Cell) error {
	for _, service := range cell.Containers.Services {
		_ = a.Runner.Run(ctx, "docker", "rm", "-f", service.ContainerName)
	}
	return nil
}

type containerInspection struct {
	Config          dockerConfig          `json:"Config"`
	Mounts          []dockerMount         `json:"Mounts"`
	NetworkSettings dockerNetworkSettings `json:"NetworkSettings"`
}

type dockerConfig struct {
	Image string   `json:"Image"`
	Env   []string `json:"Env"`
}

type dockerMount struct {
	Type        string `json:"Type"`
	Name        string `json:"Name"`
	Source      string `json:"Source"`
	Destination string `json:"Destination"`
	RW          bool   `json:"RW"`
}

type dockerNetworkSettings struct {
	Networks map[string]dockerNetwork `json:"Networks"`
}

type dockerNetwork struct{}
