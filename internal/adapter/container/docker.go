package container

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hgsg11/paracell/internal/adapter/system"
	"github.com/hgsg11/paracell/internal/domain"
)

type RunSpec struct {
	Name           string
	Image          string
	Network        string
	NetworkAliases []string
	Labels         map[string]string
	Env            []string
	Entrypoint     []string
	Command        []string
	WorkDir        string
	User           string
	Tty            bool
	OpenStdin      bool
	Health         HealthcheckSpec
	Mounts         []string
	Ports          map[string]string
	ExposedPorts   []string
}

type HealthcheckSpec struct {
	Disabled    bool
	Command     string
	Interval    time.Duration
	Timeout     time.Duration
	StartPeriod time.Duration
	Retries     int
}

func BuildDockerRunArgs(spec RunSpec) []string {
	args := []string{"run", "-d", "--name", spec.Name}
	if spec.Network != "" {
		args = append(args, "--network", spec.Network)
	}
	for _, alias := range spec.NetworkAliases {
		args = append(args, "--network-alias", alias)
	}
	labelNames := make([]string, 0, len(spec.Labels))
	for name := range spec.Labels {
		labelNames = append(labelNames, name)
	}
	sort.Strings(labelNames)
	for _, name := range labelNames {
		args = append(args, "--label", name+"="+spec.Labels[name])
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
	if spec.User != "" {
		args = append(args, "--user", spec.User)
	}
	if spec.Tty {
		args = append(args, "-t")
	}
	if spec.OpenStdin {
		args = append(args, "-i")
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
	for _, port := range spec.ExposedPorts {
		args = append(args, "-p", port)
	}
	args = appendHealthcheckArgs(args, spec.Health)
	args = append(args, spec.Image)
	args = append(args, spec.Command...)
	return args
}

func appendHealthcheckArgs(args []string, health HealthcheckSpec) []string {
	if health.Disabled {
		return append(args, "--no-healthcheck")
	}
	if health.Command == "" {
		return args
	}
	args = append(args, "--health-cmd", health.Command)
	if health.Interval > 0 {
		args = append(args, "--health-interval", health.Interval.String())
	}
	if health.Timeout > 0 {
		args = append(args, "--health-timeout", health.Timeout.String())
	}
	if health.StartPeriod > 0 {
		args = append(args, "--health-start-period", health.StartPeriod.String())
	}
	if health.Retries > 0 {
		args = append(args, "--health-retries", strconv.Itoa(health.Retries))
	}
	return args
}

type DockerCLIAdapter struct {
	Runner system.Runner
	Root   string
}

const (
	composeProjectLabel     = "com.docker.compose.project"
	composeWorkingDirLabel  = "com.docker.compose.project.working_dir"
	composeConfigFilesLabel = "com.docker.compose.project.config_files"
	composeServiceLabel     = "com.docker.compose.service"
)

func (a DockerCLIAdapter) CreateContainers(ctx context.Context, cell domain.Cell, templates []domain.ContainerTemplate) (returnErr error) {
	network := cellNetworkName(cell)
	networkCreated := false
	createdContainers := make([]string, 0, len(cell.Containers.Services))
	sharedContainers := make([]string, 0, 1)
	templatesByName := make(map[string]domain.ContainerTemplate, len(templates))
	for _, template := range templates {
		templatesByName[template.Name] = template
	}
	defer func() {
		if returnErr == nil {
			return
		}
		returnErr = errors.Join(returnErr, a.rollbackContainerStage(context.WithoutCancel(ctx), network, createdContainers, sharedContainers, networkCreated))
	}()
	if network != "" {
		if err := a.Runner.Run(ctx, "docker", "network", "create", network); err != nil {
			return err
		}
		networkCreated = true
		if err := a.ensureGateway(ctx, network); err != nil {
			return err
		}
	}
	for _, role := range sortedServiceRoles(cell.Containers.Services) {
		service := cell.Containers.Services[role]
		template := templatesByName[role]
		source := service.SourceContainer
		inspection, err := a.inspectContainer(ctx, source)
		if err != nil {
			return err
		}
		if service.Mode == domain.Dependency {
			aliases := isolatedNetworkAliases(inspection.NetworkSettings.Networks)
			if len(aliases) == 0 {
				return fmt.Errorf("dependency container %q for service %q has no usable network aliases", source, role)
			}
			if _, connected := inspection.NetworkSettings.Networks[network]; !connected {
				if err := a.connectDependency(ctx, network, source, aliases); err != nil {
					return err
				}
			}
			sharedContainers = append(sharedContainers, source)
			continue
		}
		mounts, err := a.prepareMounts(ctx, cell, service, template, inspection)
		if err != nil {
			return err
		}
		networkAliases := isolatedNetworkAliases(inspection.NetworkSettings.Networks)
		networkAliases = appendNetworkAlias(networkAliases, domain.SafeResourceName(role, "service"))
		labels := map[string]string{
			composeProjectLabel: network,
			composeServiceLabel: role,
		}
		for name, value := range gatewayLabels(cell, service.ContainerName, role, inspection.HostConfig.PortBindings) {
			labels[name] = value
		}
		args := BuildDockerRunArgs(RunSpec{
			Name:           service.ContainerName,
			Image:          inspection.Config.Image,
			Network:        network,
			NetworkAliases: networkAliases,
			Labels:         labels,
			Env:            mergeEnvironment(inspection.Config.Env, template.Environments),
			Entrypoint:     append([]string(nil), inspection.Config.Entrypoint...),
			Command:        append([]string(nil), inspection.Config.Cmd...),
			WorkDir:        inspection.Config.WorkingDir,
			User:           inspection.Config.User,
			Tty:            inspection.Config.Tty,
			OpenStdin:      inspection.Config.OpenStdin,
			Health:         inspection.Config.Healthcheck.toSpec(),
			Mounts:         mounts,
			ExposedPorts:   exposedPortsFromBindings(inspection.HostConfig.PortBindings),
		})
		if err := a.Runner.Run(ctx, "docker", args...); err != nil {
			return err
		}
		createdContainers = append(createdContainers, service.ContainerName)
	}
	return nil
}

func mergeEnvironment(source []string, overrides []domain.Environment) []string {
	merged := append([]string(nil), source...)
	if len(overrides) == 0 {
		return merged
	}

	indexes := make(map[string]int, len(merged))
	for index, entry := range merged {
		name, _, _ := strings.Cut(entry, "=")
		indexes[name] = index
	}

	for _, override := range overrides {
		entry := override.Name + "=" + override.Value
		if index, ok := indexes[override.Name]; ok {
			merged[index] = entry
			continue
		}
		merged = append(merged, entry)
	}
	return merged
}

func (a DockerCLIAdapter) connectDependency(ctx context.Context, network string, source string, aliases []string) error {
	args := []string{"network", "connect"}
	for _, alias := range aliases {
		args = append(args, "--alias", alias)
	}
	args = append(args, network, source)
	if err := a.Runner.Run(ctx, "docker", args...); err != nil {
		return fmt.Errorf("connect dependency container %q to network %q: %w", source, network, err)
	}
	return nil
}

func (a DockerCLIAdapter) disconnectDependency(ctx context.Context, network string, source string) error {
	if err := a.Runner.Run(ctx, "docker", "network", "disconnect", network, source); err != nil && !isMissingDockerNetworkConnectionError(err) {
		return fmt.Errorf("disconnect dependency container %q from network %q: %w", source, network, err)
	}
	return nil
}

func (a DockerCLIAdapter) rollbackContainerStage(ctx context.Context, network string, containers []string, sharedContainers []string, networkCreated bool) error {
	var rollbackErr error
	for i := len(containers) - 1; i >= 0; i-- {
		if err := a.Runner.Run(ctx, "docker", "rm", "-f", containers[i]); err != nil && !isMissingDockerResourceError(err) {
			rollbackErr = errors.Join(rollbackErr, err)
		}
	}
	for i := len(sharedContainers) - 1; i >= 0; i-- {
		if err := a.disconnectDependency(ctx, network, sharedContainers[i]); err != nil {
			rollbackErr = errors.Join(rollbackErr, err)
		}
	}
	if err := a.disconnectGateway(ctx, network); err != nil {
		rollbackErr = errors.Join(rollbackErr, err)
	}
	if networkCreated {
		if err := a.Runner.Run(ctx, "docker", "network", "rm", network); err != nil && !isMissingDockerResourceError(err) {
			rollbackErr = errors.Join(rollbackErr, err)
		}
	}
	return rollbackErr
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

func (a DockerCLIAdapter) prepareMounts(ctx context.Context, cell domain.Cell, service domain.CellContainer, template domain.ContainerTemplate, inspection containerInspection) ([]string, error) {
	composeMounts, err := a.resolveComposeMounts(ctx, inspection.Config.Labels)
	if err != nil {
		return nil, err
	}
	mounts, err := a.copyMounts(ctx, cell, service, inspection.Mounts, composeMounts)
	if err != nil {
		return nil, err
	}
	for _, mount := range template.Mounts {
		source := filepath.Join(a.Root, ".paracell", "cells", cell.Name, "source", mount.SourcePath)
		mounts = append(mounts, source+":"+mount.TargetPath)
	}
	return mounts, nil
}

func (a DockerCLIAdapter) cellMounts(cell domain.Cell, service domain.CellContainer, mounts []dockerMount, composeMounts *composeMountPlan) []string {
	out := make([]string, 0, len(mounts))
	for _, mount := range mounts {
		if mount.Type == "volume" && mount.Name != "" {
			out = append(out, mount.Name+":"+mount.Destination+":ro")
			continue
		}
		if mount.Type != "bind" {
			continue
		}
		spec, ok := a.bindMountSpec(cell, mount, composeMounts)
		if !ok {
			continue
		}
		out = append(out, spec)
	}
	return out
}

func (a DockerCLIAdapter) copyMounts(ctx context.Context, cell domain.Cell, service domain.CellContainer, mounts []dockerMount, composeMounts *composeMountPlan) ([]string, error) {
	out := make([]string, 0, len(mounts))
	for _, mount := range mounts {
		if mount.Type == "volume" && mount.Name != "" {
			targetVolume := copiedVolumeName(service.ContainerName, mount.Destination)
			if err := a.copyNamedVolume(ctx, mount.Name, targetVolume); err != nil {
				return nil, err
			}
			spec := targetVolume + ":" + mount.Destination
			if !mount.RW {
				spec += ":ro"
			}
			out = append(out, spec)
			continue
		}
		if mount.Type != "bind" {
			continue
		}
		spec, ok := a.bindMountSpec(cell, mount, composeMounts)
		if !ok {
			continue
		}
		out = append(out, spec)
	}
	return out, nil
}

func (a DockerCLIAdapter) bindMountSpec(cell domain.Cell, mount dockerMount, composeMounts *composeMountPlan) (string, bool) {
	source := mount.Source
	if composeMounts != nil {
		composeSource, ok := composeMounts.BindSources[mount.Destination]
		if !ok {
			return "", false
		}
		source = composeSource
		if rel, ok := composeMounts.CellSourceRelPaths[mount.Destination]; ok {
			source = a.cellSourcePath(cell)
			if rel != "." {
				source = filepath.Join(source, rel)
			}
		}
	} else {
		rel, err := filepath.Rel(a.Root, mount.Source)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", false
		}
		source = a.cellSourcePath(cell)
		if rel != "." {
			source = filepath.Join(source, rel)
		}
	}
	spec := source + ":" + mount.Destination
	if !mount.RW {
		spec += ":ro"
	}
	return spec, true
}

func (a DockerCLIAdapter) resolveComposeMounts(ctx context.Context, labels map[string]string) (*composeMountPlan, error) {
	workingDir := strings.TrimSpace(labels[composeWorkingDirLabel])
	configFiles := splitComposeConfigFiles(labels[composeConfigFilesLabel])
	serviceName := strings.TrimSpace(labels[composeServiceLabel])
	if workingDir == "" || len(configFiles) == 0 || serviceName == "" {
		return nil, nil
	}

	args := []string{"compose", "--project-directory", workingDir}
	for _, file := range configFiles {
		args = append(args, "-f", file)
	}
	args = append(args, "--profile", "*", "config", "--format", "json")
	raw, err := a.Runner.Output(ctx, "docker", args...)
	if err != nil {
		return nil, fmt.Errorf("resolve compose config for service %q: %w", serviceName, err)
	}
	var project composeProjectConfig
	if err := json.Unmarshal([]byte(raw), &project); err != nil {
		return nil, fmt.Errorf("decode compose config for service %q: %w", serviceName, err)
	}
	composeService, ok := project.Services[serviceName]
	if !ok {
		return nil, fmt.Errorf("service %q not found in compose config", serviceName)
	}
	projectRoot, err := canonicalUserPath(a.Root, "")
	if err != nil {
		return nil, err
	}
	plan := &composeMountPlan{
		BindSources:        make(map[string]string),
		CellSourceRelPaths: make(map[string]string),
	}
	for _, volume := range composeService.Volumes {
		if volume.Type != "bind" {
			continue
		}
		source, err := canonicalUserPath(volume.Source, workingDir)
		if err != nil {
			return nil, err
		}
		plan.BindSources[volume.Target] = source
		rel, err := filepath.Rel(projectRoot, source)
		if err != nil {
			return nil, fmt.Errorf("resolve compose bind source %q relative to project root %q: %w", source, projectRoot, err)
		}
		if rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			if volume.Target == "" {
				return nil, fmt.Errorf("compose bind target is empty for project source %q", source)
			}
			plan.CellSourceRelPaths[volume.Target] = rel
		}
	}
	return plan, nil
}

func splitComposeConfigFiles(value string) []string {
	parts := strings.Split(value, ",")
	files := make([]string, 0, len(parts))
	for _, part := range parts {
		if file := strings.TrimSpace(part); file != "" {
			files = append(files, file)
		}
	}
	return files
}

func canonicalUserPath(path string, base string) (string, error) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(base, path)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve user path %q: %w", path, err)
	}
	return filepath.Clean(absolute), nil
}

func (a DockerCLIAdapter) cellSourcePath(cell domain.Cell) string {
	if len(cell.Sources) == 0 {
		return ""
	}
	if filepath.IsAbs(cell.Sources[0].Path) {
		return filepath.Clean(cell.Sources[0].Path)
	}
	return filepath.Join(a.Root, cell.Sources[0].Path)
}

func (a DockerCLIAdapter) copyNamedVolume(ctx context.Context, source string, target string) error {
	if err := a.createNamedVolume(ctx, target); err != nil {
		return err
	}
	return a.Runner.Run(
		ctx,
		"docker",
		"run",
		"--rm",
		"-v", source+":/from:ro",
		"-v", target+":/to",
		"alpine",
		"sh",
		"-c",
		"cp -a /from/. /to/",
	)
}

func (a DockerCLIAdapter) createNamedVolume(ctx context.Context, name string) error {
	return a.Runner.Run(ctx, "docker", "volume", "create", name)
}

func copiedVolumeName(container string, destination string) string {
	name := strings.Trim(destination, "/")
	name = strings.ReplaceAll(name, "/", "-")
	if name == "" {
		name = "root"
	}
	return domain.SafeResourceName(container+"-"+name, "volume")
}

func cellNetworkName(cell domain.Cell) string {
	for _, service := range cell.Containers.Services {
		if idx := strings.LastIndex(service.ContainerName, "-"); idx > 0 {
			return service.ContainerName[:idx]
		}
	}
	return cell.Containers.Network
}

func isolatedNetworkAliases(networks map[string]dockerNetwork) []string {
	unique := make(map[string]struct{})
	for _, network := range networks {
		for _, alias := range network.Aliases {
			if alias != "" {
				unique[alias] = struct{}{}
			}
		}
	}
	aliases := make([]string, 0, len(unique))
	for alias := range unique {
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)
	return aliases
}

func appendNetworkAlias(aliases []string, alias string) []string {
	if alias == "" {
		return aliases
	}
	for _, existing := range aliases {
		if existing == alias {
			return aliases
		}
	}
	aliases = append(aliases, alias)
	sort.Strings(aliases)
	return aliases
}

func sortedServiceRoles(services map[string]domain.CellContainer) []string {
	roles := make([]string, 0, len(services))
	for role := range services {
		roles = append(roles, role)
	}
	sort.Strings(roles)
	return roles
}

func (a DockerCLIAdapter) CleanContainers(ctx context.Context, cell domain.Cell) error {
	var cleanupErr error
	for _, role := range sortedServiceRoles(cell.Containers.Services) {
		service := cell.Containers.Services[role]
		if service.Mode == domain.Dependency {
			continue
		}
		if err := a.Runner.Run(ctx, "docker", "rm", "-f", service.ContainerName); err != nil && !isMissingDockerResourceError(err) {
			cleanupErr = errors.Join(cleanupErr, err)
		}
	}
	if network := cellNetworkName(cell); network != "" {
		for _, role := range sortedServiceRoles(cell.Containers.Services) {
			service := cell.Containers.Services[role]
			if service.Mode != domain.Dependency {
				continue
			}
			if err := a.disconnectDependency(ctx, network, service.SourceContainer); err != nil {
				cleanupErr = errors.Join(cleanupErr, err)
			}
		}
		if err := a.disconnectGateway(ctx, network); err != nil {
			cleanupErr = errors.Join(cleanupErr, err)
		}
		if err := a.Runner.Run(ctx, "docker", "network", "rm", network); err != nil && !isMissingDockerResourceError(err) {
			cleanupErr = errors.Join(cleanupErr, err)
		}
	}
	return cleanupErr
}

func isMissingDockerResourceError(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "no such container") || strings.Contains(message, "not found")
}

func isMissingDockerNetworkConnectionError(err error) bool {
	if isMissingDockerResourceError(err) {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "is not connected to network") || strings.Contains(message, "not connected")
}

type containerInspection struct {
	Config          dockerConfig          `json:"Config"`
	HostConfig      dockerHostConfig      `json:"HostConfig"`
	Mounts          []dockerMount         `json:"Mounts"`
	NetworkSettings dockerNetworkSettings `json:"NetworkSettings"`
}

type dockerConfig struct {
	Image       string             `json:"Image"`
	Env         []string           `json:"Env"`
	Labels      map[string]string  `json:"Labels"`
	Entrypoint  []string           `json:"Entrypoint"`
	Cmd         []string           `json:"Cmd"`
	WorkingDir  string             `json:"WorkingDir"`
	User        string             `json:"User"`
	Tty         bool               `json:"Tty"`
	OpenStdin   bool               `json:"OpenStdin"`
	Healthcheck *dockerHealthcheck `json:"Healthcheck"`
}

type dockerHostConfig struct {
	PortBindings map[string][]dockerPortBinding `json:"PortBindings"`
}

type dockerPortBinding struct {
	HostIP   string `json:"HostIp"`
	HostPort string `json:"HostPort"`
}

type dockerHealthcheck struct {
	Test        []string `json:"Test"`
	Interval    int64    `json:"Interval"`
	Timeout     int64    `json:"Timeout"`
	StartPeriod int64    `json:"StartPeriod"`
	Retries     int      `json:"Retries"`
}

func (h *dockerHealthcheck) toSpec() HealthcheckSpec {
	if h == nil || len(h.Test) == 0 {
		return HealthcheckSpec{}
	}
	if len(h.Test) == 1 && h.Test[0] == "NONE" {
		return HealthcheckSpec{Disabled: true}
	}
	command := ""
	switch h.Test[0] {
	case "CMD-SHELL":
		if len(h.Test) > 1 {
			command = h.Test[1]
		}
	case "CMD":
		if len(h.Test) > 1 {
			command = strings.Join(h.Test[1:], " ")
		}
	}
	return HealthcheckSpec{
		Command:     command,
		Interval:    time.Duration(h.Interval),
		Timeout:     time.Duration(h.Timeout),
		StartPeriod: time.Duration(h.StartPeriod),
		Retries:     h.Retries,
	}
}

type dockerMount struct {
	Type        string `json:"Type"`
	Name        string `json:"Name"`
	Source      string `json:"Source"`
	Destination string `json:"Destination"`
	RW          bool   `json:"RW"`
}

type composeProjectConfig struct {
	Services map[string]composeServiceConfig `json:"services"`
}

type composeServiceConfig struct {
	Volumes []composeVolumeConfig `json:"volumes"`
}

type composeVolumeConfig struct {
	Type   string `json:"type"`
	Source string `json:"source"`
	Target string `json:"target"`
}

type composeMountPlan struct {
	BindSources        map[string]string
	CellSourceRelPaths map[string]string
}

type dockerNetworkSettings struct {
	Networks map[string]dockerNetwork       `json:"Networks"`
	Ports    map[string][]dockerPortBinding `json:"Ports"`
}

type dockerNetwork struct {
	Aliases []string `json:"Aliases"`
}

func portsFromBindings(bindings map[string][]dockerPortBinding) map[string]string {
	if len(bindings) == 0 {
		return nil
	}
	ports := map[string]string{}
	for containerPort, hostBindings := range bindings {
		for _, binding := range hostBindings {
			if binding.HostPort == "" {
				continue
			}
			host := binding.HostPort
			if binding.HostIP != "" {
				host = binding.HostIP + ":" + host
			}
			ports[host] = strings.TrimSuffix(containerPort, "/tcp")
		}
	}
	if len(ports) == 0 {
		return nil
	}
	return ports
}

func exposedPortsFromBindings(bindings map[string][]dockerPortBinding) []string {
	if len(bindings) == 0 {
		return nil
	}
	ports := make([]string, 0, len(bindings))
	for containerPort := range bindings {
		ports = append(ports, strings.TrimSuffix(containerPort, "/tcp"))
	}
	sort.Strings(ports)
	if len(ports) == 0 {
		return nil
	}
	return ports
}
