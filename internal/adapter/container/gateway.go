package container

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/hgsg11/paracell/internal/domain"
)

const (
	gatewayContainerName      = "paracell-gateway"
	gatewayImage              = "traefik:v3.7"
	gatewayManagedLabel       = "io.paracell.gateway"
	gatewayConfigVersionLabel = "io.paracell.gateway.config-version"
	gatewayConfigVersion      = "2"
	gatewayDashboardRouter    = "paracell-gateway-dashboard"
	gatewayDashboardHost      = "gateway.paracell.localhost"
	gatewayReplacementName    = "paracell-gateway-replaced"
)

type gatewayInspection struct {
	Config     dockerConfig     `json:"Config"`
	HostConfig dockerHostConfig `json:"HostConfig"`
	State      struct {
		Running bool `json:"Running"`
	} `json:"State"`
	NetworkSettings dockerNetworkSettings `json:"NetworkSettings"`
}

func gatewayLabels(cell domain.Cell, containerName string, aliases []string, bindings map[string][]dockerPortBinding) map[string]string {
	ports := publishedTCPPorts(bindings)
	if len(aliases) == 0 || len(ports) == 0 {
		return nil
	}

	project := gatewayProjectName(cell)
	cellName := gatewayHostLabel(cell.Name)
	if project == "" || cellName == "" {
		return nil
	}

	hostAliases := make([]string, 0, len(aliases))
	seenAliases := make(map[string]struct{}, len(aliases))
	for _, alias := range aliases {
		hostAlias := gatewayHostLabel(alias)
		if hostAlias == "" {
			continue
		}
		if _, ok := seenAliases[hostAlias]; ok {
			continue
		}
		seenAliases[hostAlias] = struct{}{}
		hostAliases = append(hostAliases, hostAlias)
	}
	if len(hostAliases) == 0 {
		return nil
	}
	sort.Strings(hostAliases)

	labels := map[string]string{
		"traefik.enable":         "true",
		"traefik.docker.network": cellNetworkName(cell),
	}
	for _, port := range ports {
		name := gatewayRouteName(containerName, port)
		hosts := make([]string, 0, len(hostAliases))
		for _, alias := range hostAliases {
			prefix := alias
			if len(ports) > 1 {
				prefix = "p" + port + "." + prefix
			}
			hosts = append(hosts, fmt.Sprintf("Host(`%s.%s.%s.localhost`)", prefix, cellName, project))
		}
		router := "traefik.http.routers." + name
		service := "traefik.http.services." + name
		labels[router+".entrypoints"] = "web"
		labels[router+".rule"] = strings.Join(hosts, " || ")
		labels[router+".service"] = name
		labels[service+".loadbalancer.server.port"] = port
	}
	return labels
}

func publishedTCPPorts(bindings map[string][]dockerPortBinding) []string {
	ports := make([]string, 0, len(bindings))
	seen := make(map[string]struct{}, len(bindings))
	for binding := range bindings {
		port, protocol, found := strings.Cut(binding, "/")
		if found && protocol != "tcp" {
			continue
		}
		if port == "" {
			continue
		}
		if _, err := strconv.ParseUint(port, 10, 16); err != nil {
			continue
		}
		if _, ok := seen[port]; ok {
			continue
		}
		seen[port] = struct{}{}
		ports = append(ports, port)
	}
	sort.Slice(ports, func(i, j int) bool {
		left, _ := strconv.Atoi(ports[i])
		right, _ := strconv.Atoi(ports[j])
		return left < right
	})
	return ports
}

func gatewayProjectName(cell domain.Cell) string {
	network := strings.TrimPrefix(cellNetworkName(cell), "paracell-")
	cellSuffix := "-" + cell.Name
	if strings.HasSuffix(network, cellSuffix) {
		network = strings.TrimSuffix(network, cellSuffix)
	}
	return gatewayHostLabel(network)
}

func gatewayHostLabel(value string) string {
	value = strings.ToLower(value)
	var result strings.Builder
	separator := false
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-' {
			if separator && result.Len() > 0 {
				result.WriteByte('-')
			}
			result.WriteRune(r)
			separator = false
			continue
		}
		separator = true
	}
	return strings.Trim(result.String(), "-")
}

func gatewayRouteName(containerName string, port string) string {
	return strings.ReplaceAll(containerName+"-p"+port, "@", "-")
}

func (a DockerCLIAdapter) ensureGateway(ctx context.Context, network string) error {
	raw, err := a.Runner.Output(ctx, "docker", "inspect", "-f", "{{json .}}", gatewayContainerName)
	if err != nil {
		if err := a.Runner.Run(ctx, "docker", gatewayRunArgs("127.0.0.1:80:80")...); err != nil {
			if !isGatewayPortConflict(err) {
				return fmt.Errorf("start Paracell gateway on 127.0.0.1:80: %w", err)
			}
			if removeErr := a.Runner.Run(ctx, "docker", "rm", "-f", gatewayContainerName); removeErr != nil && !isMissingDockerResourceError(removeErr) {
				return fmt.Errorf("remove Paracell gateway after port 80 conflict: %w", removeErr)
			}
			if fallbackErr := a.Runner.Run(ctx, "docker", gatewayRunArgs("127.0.0.1::80")...); fallbackErr != nil {
				return fmt.Errorf("start Paracell gateway on an available 127.0.0.1 port: %w", fallbackErr)
			}
		}
		if err := a.Runner.Run(ctx, "docker", "network", "connect", network, gatewayContainerName); err != nil {
			return fmt.Errorf("connect Paracell gateway to network %q: %w", network, err)
		}
		return nil
	}

	var inspection gatewayInspection
	if err := json.Unmarshal([]byte(raw), &inspection); err != nil {
		return fmt.Errorf("inspect Paracell gateway: %w", err)
	}
	if inspection.Config.Labels[gatewayManagedLabel] != "true" {
		return fmt.Errorf("container %q already exists and is not managed by Paracell", gatewayContainerName)
	}
	if inspection.Config.Labels[gatewayConfigVersionLabel] != gatewayConfigVersion {
		if err := a.recreateGateway(ctx, inspection); err != nil {
			return err
		}
		if _, connected := inspection.NetworkSettings.Networks[network]; connected {
			return nil
		}
		return a.connectGateway(ctx, network)
	}
	if !inspection.State.Running {
		if err := a.Runner.Run(ctx, "docker", "start", gatewayContainerName); err != nil {
			return fmt.Errorf("start Paracell gateway on 127.0.0.1:80: %w", err)
		}
	}
	if _, connected := inspection.NetworkSettings.Networks[network]; connected {
		return nil
	}
	return a.connectGateway(ctx, network)
}

func gatewayRunArgs(publish string) []string {
	router := "traefik.http.routers." + gatewayDashboardRouter
	return []string{
		"run", "-d",
		"--name", gatewayContainerName,
		"--label", gatewayManagedLabel + "=true",
		"--label", gatewayConfigVersionLabel + "=" + gatewayConfigVersion,
		"--label", "traefik.enable=true",
		"--label", router + ".entrypoints=web",
		"--label", router + ".rule=Host(`" + gatewayDashboardHost + "`) && (PathPrefix(`/dashboard/`) || PathPrefix(`/api`))",
		"--label", router + ".service=api@internal",
		"--restart", "unless-stopped",
		"-p", publish,
		"-v", "/var/run/docker.sock:/var/run/docker.sock:ro",
		gatewayImage,
		"--providers.docker=true",
		"--providers.docker.exposedbydefault=false",
		"--entrypoints.web.address=:80",
		"--api.dashboard=true",
	}
}

func (a DockerCLIAdapter) recreateGateway(ctx context.Context, inspection gatewayInspection) error {
	publish := gatewayPublish(inspection)
	networks := gatewayCellNetworks(inspection.NetworkSettings.Networks)
	if err := a.Runner.Run(ctx, "docker", "rename", gatewayContainerName, gatewayReplacementName); err != nil {
		return fmt.Errorf("preserve outdated Paracell gateway before replacement: %w", err)
	}
	if err := a.Runner.Run(ctx, "docker", "stop", gatewayReplacementName); err != nil {
		restoreErr := a.Runner.Run(ctx, "docker", "rename", gatewayReplacementName, gatewayContainerName)
		return errors.Join(fmt.Errorf("stop outdated Paracell gateway: %w", err), restoreErr)
	}
	if err := a.Runner.Run(ctx, "docker", gatewayRunArgs(publish)...); err != nil {
		return errors.Join(fmt.Errorf("recreate Paracell gateway on %s: %w", publish, err), a.restoreReplacedGateway(ctx, inspection.State.Running))
	}
	for _, network := range networks {
		if err := a.connectGateway(ctx, network); err != nil {
			return errors.Join(err, a.restoreReplacedGateway(ctx, inspection.State.Running))
		}
	}
	if err := a.Runner.Run(ctx, "docker", "rm", gatewayReplacementName); err != nil {
		return fmt.Errorf("remove replaced Paracell gateway: %w", err)
	}
	return nil
}

func (a DockerCLIAdapter) restoreReplacedGateway(ctx context.Context, wasRunning bool) error {
	var restoreErr error
	if err := a.Runner.Run(ctx, "docker", "rm", "-f", gatewayContainerName); err != nil && !isMissingDockerResourceError(err) {
		restoreErr = errors.Join(restoreErr, err)
	}
	if err := a.Runner.Run(ctx, "docker", "rename", gatewayReplacementName, gatewayContainerName); err != nil {
		return errors.Join(restoreErr, err)
	}
	if wasRunning {
		if err := a.Runner.Run(ctx, "docker", "start", gatewayContainerName); err != nil {
			restoreErr = errors.Join(restoreErr, err)
		}
	}
	return restoreErr
}

func gatewayPublish(inspection gatewayInspection) string {
	bindings := inspection.NetworkSettings.Ports["80/tcp"]
	if len(bindings) == 0 {
		bindings = inspection.HostConfig.PortBindings["80/tcp"]
	}
	for _, binding := range bindings {
		if binding.HostPort != "" && (binding.HostIP == "127.0.0.1" || binding.HostIP == "") {
			return "127.0.0.1:" + binding.HostPort + ":80"
		}
	}
	return "127.0.0.1:80:80"
}

func gatewayCellNetworks(networks map[string]dockerNetwork) []string {
	names := make([]string, 0, len(networks))
	for name := range networks {
		if strings.HasPrefix(name, "paracell-") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func (a DockerCLIAdapter) connectGateway(ctx context.Context, network string) error {
	if err := a.Runner.Run(ctx, "docker", "network", "connect", network, gatewayContainerName); err != nil {
		return fmt.Errorf("connect Paracell gateway to network %q: %w", network, err)
	}
	return nil
}

func isGatewayPortConflict(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "port is already allocated") || strings.Contains(message, "address already in use")
}

func (a DockerCLIAdapter) disconnectGateway(ctx context.Context, network string) error {
	err := a.Runner.Run(ctx, "docker", "network", "disconnect", "-f", network, gatewayContainerName)
	if err == nil || isMissingDockerResourceError(err) || strings.Contains(strings.ToLower(err.Error()), "not connected") {
		return nil
	}
	return err
}
