package container

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/hgsg11/paracell/internal/domain"
)

const (
	gatewayContainerName = "paracell-gateway"
	gatewayImage         = "traefik:v3.7"
	gatewayManagedLabel  = "io.paracell.gateway"
)

type gatewayInspection struct {
	Config dockerConfig `json:"Config"`
	State  struct {
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
	if !inspection.State.Running {
		if err := a.Runner.Run(ctx, "docker", "start", gatewayContainerName); err != nil {
			return fmt.Errorf("start Paracell gateway on 127.0.0.1:80: %w", err)
		}
	}
	if _, connected := inspection.NetworkSettings.Networks[network]; connected {
		return nil
	}
	if err := a.Runner.Run(ctx, "docker", "network", "connect", network, gatewayContainerName); err != nil {
		return fmt.Errorf("connect Paracell gateway to network %q: %w", network, err)
	}
	return nil
}

func gatewayRunArgs(publish string) []string {
	return []string{
		"run", "-d",
		"--name", gatewayContainerName,
		"--label", gatewayManagedLabel + "=true",
		"--restart", "unless-stopped",
		"-p", publish,
		"-v", "/var/run/docker.sock:/var/run/docker.sock:ro",
		gatewayImage,
		"--providers.docker=true",
		"--providers.docker.exposedbydefault=false",
		"--entrypoints.web.address=:80",
	}
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
