package container

import (
	"context"
	"sort"

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
}

func (a DockerCLIAdapter) CreateContainers(ctx context.Context, cell domain.Cell, template domain.Template) error {
	if err := a.Runner.Run(ctx, "docker", "network", "create", cell.Containers.Network); err != nil {
		return err
	}
	for role, service := range cell.Containers.Services {
		source := template.Containers.Services[role].SourceContainer
		if source == "" {
			source = service.SourceContainer
		}
		image, err := a.Runner.Output(ctx, "docker", "inspect", "-f", "{{.Config.Image}}", source)
		if err != nil {
			return err
		}
		args := BuildDockerRunArgs(RunSpec{
			Name:    service.ContainerName,
			Image:   image,
			Network: cell.Containers.Network,
		})
		if err := a.Runner.Run(ctx, "docker", args...); err != nil {
			return err
		}
	}
	return nil
}

func (a DockerCLIAdapter) RemoveContainers(ctx context.Context, cell domain.Cell) error {
	for _, service := range cell.Containers.Services {
		_ = a.Runner.Run(ctx, "docker", "rm", "-f", service.ContainerName)
	}
	return a.Runner.Run(ctx, "docker", "network", "rm", cell.Containers.Network)
}
