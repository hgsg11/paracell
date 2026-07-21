package container

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
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
	composeWorkingDirLabel  = "com.docker.compose.project.working_dir"
	composeConfigFilesLabel = "com.docker.compose.project.config_files"
	composeServiceLabel     = "com.docker.compose.service"
)

func (a DockerCLIAdapter) CreateContainers(ctx context.Context, cell domain.Cell, template domain.Template) error {
	network := cellNetworkName(cell)
	if shouldCreateIsolatedNetwork(cell.Containers.NetworkMode) && network != "" {
		if err := a.Runner.Run(ctx, "docker", "network", "create", network); err != nil {
			return err
		}
	}
	for _, role := range sortedServiceRoles(cell.Containers.Services) {
		service := cell.Containers.Services[role]
		source := template.Containers.Services[role].SourceContainer
		if source == "" {
			source = service.SourceContainer
		}
		inspection, err := a.inspectContainer(ctx, source)
		if err != nil {
			return err
		}
		mounts, err := a.prepareMounts(ctx, cell, service, inspection)
		if err != nil {
			return err
		}
		runNetwork := network
		extraNetworks := []string(nil)
		networkAliases := []string(nil)
		if cell.Containers.NetworkMode == string(domain.ContainerNetworkShared) {
			runNetwork, extraNetworks = sharedNetworks(inspection.NetworkSettings.Networks)
		} else {
			networkAliases = isolatedNetworkAliases(inspection.NetworkSettings.Networks)
		}
		args := BuildDockerRunArgs(RunSpec{
			Name:           service.ContainerName,
			Image:          inspection.Config.Image,
			Network:        runNetwork,
			NetworkAliases: networkAliases,
			Env:            append([]string(nil), inspection.Config.Env...),
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
		for _, extraNetwork := range extraNetworks {
			if err := a.Runner.Run(ctx, "docker", "network", "connect", extraNetwork, service.ContainerName); err != nil {
				return err
			}
		}
		if err := a.copyDatabase(ctx, role, source, service, inspection); err != nil {
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

func (a DockerCLIAdapter) prepareMounts(ctx context.Context, cell domain.Cell, service domain.CellContainer, inspection containerInspection) ([]string, error) {
	composeMounts, err := a.resolveComposeMounts(ctx, inspection.Config.Labels)
	if err != nil {
		return nil, err
	}
	if service.VolumeMode != "copy" {
		return a.cellMounts(cell, service, inspection.Mounts, composeMounts), nil
	}
	return a.copyMounts(ctx, cell, service, inspection.Mounts, composeMounts)
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
	out = append(out, a.initFileMounts(cell, service)...)
	return out
}

func (a DockerCLIAdapter) copyMounts(ctx context.Context, cell domain.Cell, service domain.CellContainer, mounts []dockerMount, composeMounts *composeMountPlan) ([]string, error) {
	out := make([]string, 0, len(mounts))
	for _, mount := range mounts {
		if mount.Type == "volume" && mount.Name != "" {
			targetVolume := copiedVolumeName(service.ContainerName, mount.Destination)
			if service.Database != nil && service.Database.CopyMode == "schema" {
				if err := a.createNamedVolume(ctx, targetVolume); err != nil {
					return nil, err
				}
				out = append(out, targetVolume+":"+mount.Destination+":rw")
				continue
			}
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
	out = append(out, a.initFileMounts(cell, service)...)
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
		if mount.Destination == composeMounts.SourceTarget {
			source = a.cellSourcePath(cell)
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
	plan := &composeMountPlan{BindSources: make(map[string]string)}
	for _, volume := range composeService.Volumes {
		if volume.Type != "bind" {
			continue
		}
		source, err := canonicalUserPath(volume.Source, workingDir)
		if err != nil {
			return nil, err
		}
		plan.BindSources[volume.Target] = source
		if source == projectRoot {
			if volume.Target == "" {
				return nil, fmt.Errorf("compose bind target is empty for project root %q", projectRoot)
			}
			plan.SourceTarget = volume.Target
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

func (a DockerCLIAdapter) initFileMounts(cell domain.Cell, service domain.CellContainer) []string {
	if service.Database == nil {
		return nil
	}
	out := make([]string, 0, len(service.Database.InitFiles))
	for _, file := range service.Database.InitFiles {
		clean := filepath.Clean(file)
		source := filepath.Join(a.cellSourcePath(cell), clean)
		target := filepath.Join("/docker-entrypoint-initdb.d", filepath.Base(clean))
		out = append(out, source+":"+target+":ro")
	}
	return out
}

func (a DockerCLIAdapter) cellSourcePath(cell domain.Cell) string {
	if filepath.IsAbs(cell.Source.Path) {
		return filepath.Clean(cell.Source.Path)
	}
	return filepath.Join(a.Root, cell.Source.Path)
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

func (a DockerCLIAdapter) copyDatabase(ctx context.Context, role string, source string, service domain.CellContainer, inspection containerInspection) error {
	if service.Database == nil {
		return nil
	}
	switch service.Database.CopyMode {
	case "":
		return nil
	case "schema":
		switch service.Database.System {
		case "mysql":
			return a.copyMySQLSchema(ctx, source, role, service, inspection)
		default:
			return fmt.Errorf("unsupported databaseSystem %q for service %q", service.Database.System, role)
		}
	case "data":
		return fmt.Errorf("copyMode %q is not implemented for service %q", service.Database.CopyMode, role)
	default:
		return fmt.Errorf("unsupported copyMode %q for service %q", service.Database.CopyMode, role)
	}
}

func (a DockerCLIAdapter) copyMySQLSchema(ctx context.Context, source string, role string, service domain.CellContainer, inspection containerInspection) error {
	conn, err := mysqlConnectionFromEnv(inspection.Config.Env)
	if err != nil {
		return fmt.Errorf("mysql schema dump failed for service %q: %w", role, err)
	}
	if err := a.waitForMySQL(ctx, service.ContainerName, conn); err != nil {
		return fmt.Errorf("mysql schema import failed for service %q: %w", role, err)
	}
	databasesOutput, err := a.Runner.Output(ctx, "docker", mysqlDatabaseListArgs(source, conn)...)
	if err != nil {
		return fmt.Errorf("mysql database list failed for service %q: %w", role, err)
	}
	databases := mysqlDatabaseNames(databasesOutput)
	if len(databases) == 0 {
		return nil
	}
	schema, err := a.Runner.Output(ctx, "docker", mysqlDumpArgs(source, conn, databases)...)
	if err != nil {
		return fmt.Errorf("mysql schema dump failed for service %q: %w", role, err)
	}
	temp, err := os.CreateTemp("", "paracell-schema-*.sql")
	if err != nil {
		return fmt.Errorf("mysql schema import failed for service %q: %w", role, err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if _, err := temp.WriteString(schema); err != nil {
		temp.Close()
		return fmt.Errorf("mysql schema import failed for service %q: %w", role, err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("mysql schema import failed for service %q: %w", role, err)
	}
	const containerSchemaPath = "/tmp/paracell-schema.sql"
	if err := a.Runner.Run(ctx, "docker", "cp", tempPath, service.ContainerName+":"+containerSchemaPath); err != nil {
		return fmt.Errorf("mysql schema import failed for service %q: %w", role, err)
	}
	importCommand := fmt.Sprintf("mysql -u %s", shQuote(conn.User))
	if conn.Password != "" {
		importCommand += " " + shQuote("-p"+conn.Password)
	}
	importCommand += fmt.Sprintf(" < %s", shQuote(containerSchemaPath))
	if err := a.Runner.Run(ctx, "docker", "exec", service.ContainerName, "sh", "-c", importCommand); err != nil {
		return fmt.Errorf("mysql schema import failed for service %q: %w", role, err)
	}
	if err := a.Runner.Run(ctx, "docker", "exec", service.ContainerName, "rm", "-f", containerSchemaPath); err != nil {
		return fmt.Errorf("mysql schema import failed for service %q: %w", role, err)
	}
	return nil
}

func (a DockerCLIAdapter) waitForMySQL(ctx context.Context, container string, conn mysqlConnection) error {
	args := []string{"exec", container, "mysqladmin", "ping", "-h", "127.0.0.1", "-u", conn.User}
	if conn.Password != "" {
		args = append(args, "-p"+conn.Password)
	}
	args = append(args, "--silent")
	var lastErr error
	for i := 0; i < 60; i++ {
		if err := a.Runner.Run(ctx, "docker", args...); err == nil {
			return nil
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
	return lastErr
}

type mysqlConnection struct {
	User     string
	Password string
	Database string
}

func mysqlConnectionFromEnv(env []string) (mysqlConnection, error) {
	values := make(map[string]string, len(env))
	for _, entry := range env {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		values[key] = value
	}
	conn := mysqlConnection{
		User:     values["MYSQL_USER"],
		Password: values["MYSQL_PASSWORD"],
		Database: values["MYSQL_DATABASE"],
	}
	if values["MYSQL_ROOT_PASSWORD"] != "" {
		conn.User = "root"
		conn.Password = values["MYSQL_ROOT_PASSWORD"]
	} else if conn.User == "" {
		conn.User = "root"
		conn.Password = values["MYSQL_ROOT_PASSWORD"]
	}
	if conn.User == "" || conn.Database == "" {
		return mysqlConnection{}, fmt.Errorf("MYSQL_USER/MYSQL_DATABASE or MYSQL_ROOT_PASSWORD/MYSQL_DATABASE is required")
	}
	return conn, nil
}

func mysqlDatabaseListArgs(container string, conn mysqlConnection) []string {
	args := []string{"exec", container, "mysql", "--batch", "--skip-column-names", "-u", conn.User}
	if conn.Password != "" {
		args = append(args, "-p"+conn.Password)
	}
	return append(args, "-e", "SELECT SCHEMA_NAME FROM INFORMATION_SCHEMA.SCHEMATA WHERE SCHEMA_NAME NOT IN ('information_schema', 'mysql', 'performance_schema', 'sys') ORDER BY SCHEMA_NAME")
}

func mysqlDatabaseNames(output string) []string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	databases := make([]string, 0, len(lines))
	for _, line := range lines {
		if name := strings.TrimSpace(line); name != "" {
			databases = append(databases, name)
		}
	}
	return databases
}

func mysqlDumpArgs(container string, conn mysqlConnection, databases []string) []string {
	args := []string{"exec", container, "mysqldump", "--no-data", "--no-tablespaces", "-u", conn.User}
	if conn.Password != "" {
		args = append(args, "-p"+conn.Password)
	}
	args = append(args, "--databases")
	args = append(args, databases...)
	return args
}

func shQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
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

func shouldCreateIsolatedNetwork(mode string) bool {
	return mode == "" || mode == string(domain.ContainerNetworkIsolated)
}

func sharedNetworks(networks map[string]dockerNetwork) (string, []string) {
	names := make([]string, 0, len(networks))
	for name := range networks {
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) == 0 {
		return "", nil
	}
	return names[0], names[1:]
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

func sortedServiceRoles(services map[string]domain.CellContainer) []string {
	roles := make([]string, 0, len(services))
	for role := range services {
		roles = append(roles, role)
	}
	sort.Strings(roles)
	return roles
}

func (a DockerCLIAdapter) CleanContainers(ctx context.Context, cell domain.Cell) error {
	for _, role := range sortedServiceRoles(cell.Containers.Services) {
		service := cell.Containers.Services[role]
		if err := a.Runner.Run(ctx, "docker", "rm", "-f", service.ContainerName); err != nil && !isMissingDockerResourceError(err) {
			return err
		}
	}
	if cell.Containers.NetworkMode == string(domain.ContainerNetworkIsolated) || cell.Containers.NetworkMode == "" {
		if network := cellNetworkName(cell); network != "" {
			if err := a.Runner.Run(ctx, "docker", "network", "rm", network); err != nil && !isMissingDockerResourceError(err) {
				return err
			}
		}
	}
	return nil
}

func isMissingDockerResourceError(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "no such container") || strings.Contains(message, "not found")
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
	SourceTarget string
	BindSources  map[string]string
}

type dockerNetworkSettings struct {
	Networks map[string]dockerNetwork `json:"Networks"`
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
