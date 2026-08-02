package container

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/hgsg11/paracell/internal/adapter/system"
	"github.com/hgsg11/paracell/internal/domain"
)

func TestDockerIntegrationはIsolatedCellを共有Gateway経由でRoutingする(t *testing.T) {
	if os.Getenv("PARACELL_RUN_DOCKER_TESTS") != "1" {
		t.Skip("set PARACELL_RUN_DOCKER_TESTS=1 to run Docker integration tests")
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skipf("Docker is not available: %v", err)
	}
	for _, image := range []string{"nginx:alpine", gatewayImage} {
		if err := exec.Command("docker", "image", "inspect", image).Run(); err != nil {
			t.Skipf("%s image is required: %v", image, err)
		}
	}

	removeGateway := false
	if output, err := exec.Command("docker", "inspect", "-f", "{{index .Config.Labels \"io.paracell.gateway\"}} {{.Config.Image}}", gatewayContainerName).Output(); err == nil {
		if strings.TrimSpace(string(output)) != "true "+gatewayImage {
			t.Skipf("container %s already exists but is not the managed %s image", gatewayContainerName, gatewayImage)
		}
	} else {
		removeGateway = true
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	cellName := "gateway-" + suffix
	cellNetwork := "paracell-integrationgw-" + cellName
	sourceNetwork := cellNetwork + "-source"
	sourceContainer := cellNetwork + "-source"
	targetContainer := cellNetwork + "-web"
	adapter := DockerCLIAdapter{Runner: system.CaptureRunner{}}
	cell := domain.Cell{
		Name: cellName,
		Containers: domain.Containers{
			Network:     cellNetwork,
			NetworkMode: "isolated",
			Services: map[string]domain.CellContainer{
				"web": {ContainerName: targetContainer, SourceContainer: sourceContainer},
			},
		},
	}
	template := domain.Template{Containers: domain.ContainerTemplate{Services: map[string]domain.ContainerServiceTemplate{
		"web": {SourceContainer: sourceContainer},
	}}}

	t.Cleanup(func() {
		_ = adapter.CleanContainers(context.Background(), cell)
		_ = exec.Command("docker", "rm", "-f", sourceContainer).Run()
		_ = exec.Command("docker", "network", "rm", sourceNetwork).Run()
		if removeGateway {
			_ = exec.Command("docker", "rm", "-f", gatewayContainerName).Run()
		}
	})

	runDockerIntegrationCommand(t, "network", "create", sourceNetwork)
	runDockerIntegrationCommand(t,
		"run", "-d", "--name", sourceContainer,
		"--network", sourceNetwork,
		"--network-alias", "web",
		"-p", "80",
		"nginx:alpine",
	)
	if err := adapter.CreateContainers(context.Background(), cell, template); err != nil {
		t.Fatalf("CreateContainers failed: %v", err)
	}
	gatewayEndpoint := strings.Split(dockerIntegrationOutput(t, "port", gatewayContainerName, "80/tcp"), "\n")[0]

	host := "web." + cellName + ".integrationgw.localhost"
	deadline := time.Now().Add(15 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		req, err := http.NewRequest(http.MethodGet, "http://"+gatewayEndpoint+"/", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Host = host
		response, err := http.DefaultClient.Do(req)
		if err == nil {
			body, readErr := io.ReadAll(response.Body)
			_ = response.Body.Close()
			if readErr == nil && response.StatusCode == http.StatusOK && strings.Contains(string(body), "Welcome to nginx") {
				verifyGatewayDashboard(t, gatewayEndpoint)
				return
			}
			lastErr = fmt.Errorf("status=%s body=%q readErr=%v", response.Status, strings.TrimSpace(string(body)), readErr)
		} else {
			lastErr = err
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("gateway did not route host %q to %q: %v", host, targetContainer, lastErr)
}

func verifyGatewayDashboard(t *testing.T, gatewayEndpoint string) {
	t.Helper()
	for _, path := range []string{"/dashboard/", "/api/overview", "/metrics"} {
		deadline := time.Now().Add(15 * time.Second)
		var lastErr error
		available := false
		for time.Now().Before(deadline) {
			req, err := http.NewRequest(http.MethodGet, "http://"+gatewayEndpoint+path, nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Host = gatewayDashboardHost
			response, err := http.DefaultClient.Do(req)
			if err == nil {
				body, readErr := io.ReadAll(response.Body)
				_ = response.Body.Close()
				if readErr == nil && response.StatusCode == http.StatusOK && len(body) > 0 {
					switch path {
					case "/api/overview":
						var overview struct {
							Features struct {
								Tracing   string `json:"tracing"`
								Metrics   string `json:"metrics"`
								AccessLog bool   `json:"accessLog"`
							} `json:"features"`
						}
						if err := json.Unmarshal(body, &overview); err == nil && overview.Features.Tracing != "" && overview.Features.Metrics != "" && overview.Features.AccessLog {
							available = true
						}
					case "/metrics":
						available = strings.Contains(string(body), "traefik_")
					default:
						available = true
					}
					if available {
						break
					}
				}
				lastErr = fmt.Errorf("status=%s body=%q readErr=%v", response.Status, strings.TrimSpace(string(body)), readErr)
			} else {
				lastErr = err
			}
			time.Sleep(250 * time.Millisecond)
		}
		if !available {
			t.Fatalf("gateway dashboard path %q was not available: %v", path, lastErr)
		}
	}
}

func TestDockerIntegrationはIsolatedNetworkへAliasをコピーする(t *testing.T) {
	if os.Getenv("PARACELL_RUN_DOCKER_TESTS") != "1" {
		t.Skip("set PARACELL_RUN_DOCKER_TESTS=1 to run Docker integration tests")
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skipf("Docker is not available: %v", err)
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	prefix := "paracell-integration-alias-" + suffix
	sourceNetwork := prefix + "-source-network"
	sourceContainer := prefix + "-source"
	targetContainer := prefix + "-web"
	targetNetwork := prefix

	t.Cleanup(func() {
		_ = exec.Command("docker", "rm", "-f", targetContainer, sourceContainer).Run()
		_ = exec.Command("docker", "network", "rm", targetNetwork, sourceNetwork).Run()
	})

	runDockerIntegrationCommand(t, "network", "create", sourceNetwork)
	runDockerIntegrationCommand(t,
		"run", "-d", "--name", sourceContainer,
		"--network", sourceNetwork,
		"--network-alias", "web",
		"--network-alias", "frontend",
		"alpine:3.20", "sleep", "300",
	)

	cell := domain.Cell{
		Name: "integration",
		Containers: domain.Containers{
			NetworkMode: "isolated",
			Services: map[string]domain.CellContainer{
				"web": {ContainerName: targetContainer, SourceContainer: sourceContainer},
			},
		},
	}
	template := domain.Template{Containers: domain.ContainerTemplate{Services: map[string]domain.ContainerServiceTemplate{
		"web": {SourceContainer: sourceContainer},
	}}}
	adapter := DockerCLIAdapter{Runner: system.OSCommandRunner{}}
	if err := adapter.CreateContainers(context.Background(), cell, template); err != nil {
		t.Fatalf("CreateContainers failed: %v", err)
	}

	rawAliases := dockerIntegrationOutput(t, "inspect", "-f", "{{range .NetworkSettings.Networks}}{{json .Aliases}}{{end}}", targetContainer)
	var aliases []string
	if err := json.Unmarshal([]byte(rawAliases), &aliases); err != nil {
		t.Fatalf("unmarshal target aliases %q: %v", rawAliases, err)
	}
	for _, want := range []string{"web", "frontend"} {
		if !containsString(aliases, want) {
			t.Fatalf("target aliases = %#v, want %q", aliases, want)
		}
	}

	resolved := dockerIntegrationOutput(t, "run", "--rm", "--network", targetNetwork, "alpine:3.20", "getent", "hosts", "web")
	targetIP := dockerIntegrationOutput(t, "inspect", "-f", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}", targetContainer)
	resolvedFields := strings.Fields(resolved)
	if len(resolvedFields) == 0 || resolvedFields[0] != targetIP {
		t.Fatalf("alias web resolved to %q, want target IP %q", resolved, targetIP)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestCreateContainersは同一Network上でHostPort競合せずに起動できる(t *testing.T) {
	if os.Getenv("PARACELL_RUN_DOCKER_TESTS") != "1" {
		t.Skip("set PARACELL_RUN_DOCKER_TESTS=1 to run Docker integration tests")
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skipf("docker is not available: %v", err)
	}
	if err := exec.Command("docker", "image", "inspect", "nginx:alpine").Run(); err != nil {
		t.Skipf("nginx:alpine image is required: %v", err)
	}

	ctx := context.Background()
	runner := system.OSCommandRunner{}
	network := "paracell-port-conflict-test-net"
	sourceContainer := "paracell-port-conflict-source"
	copyA := "paracell-port-conflict-copy-a"
	copyB := "paracell-port-conflict-copy-b"

	cleanup := []string{copyA, copyB, sourceContainer}
	for _, name := range cleanup {
		defer exec.Command("docker", "rm", "-f", name).Run()
	}
	defer exec.Command("docker", "network", "rm", network).Run()

	_ = exec.Command("docker", "rm", "-f", sourceContainer).Run()
	_ = exec.Command("docker", "rm", "-f", copyA).Run()
	_ = exec.Command("docker", "rm", "-f", copyB).Run()
	_ = exec.Command("docker", "network", "rm", network).Run()

	if err := runner.Run(ctx, "docker", "network", "create", network); err != nil {
		t.Fatalf("network create failed: %v", err)
	}
	if err := runner.Run(ctx, "docker", "run", "-d", "--name", sourceContainer, "--network", network, "-p", "80", "nginx:alpine"); err != nil {
		t.Fatalf("source container run failed: %v", err)
	}

	adapter := DockerCLIAdapter{Runner: runner}
	template := domain.Template{
		Containers: domain.ContainerTemplate{
			Network: "shared",
			Services: map[string]domain.ContainerServiceTemplate{
				"web": {SourceContainer: sourceContainer},
			},
		},
	}

	cellA := domain.Cell{
		Name: "copy-a",
		Containers: domain.Containers{
			NetworkMode: "shared",
			Network:     "shared",
			Services: map[string]domain.CellContainer{
				"web": {ContainerName: copyA, SourceContainer: sourceContainer},
			},
		},
	}
	cellB := domain.Cell{
		Name: "copy-b",
		Containers: domain.Containers{
			NetworkMode: "shared",
			Network:     "shared",
			Services: map[string]domain.CellContainer{
				"web": {ContainerName: copyB, SourceContainer: sourceContainer},
			},
		},
	}

	if err := adapter.CreateContainers(ctx, cellA, template); err != nil {
		t.Fatalf("CreateContainers copyA failed: %v", err)
	}
	if err := adapter.CreateContainers(ctx, cellB, template); err != nil {
		t.Fatalf("CreateContainers copyB failed: %v", err)
	}

	portA, err := runner.Output(ctx, "docker", "port", copyA, "80/tcp")
	if err != nil {
		t.Fatalf("docker port copyA failed: %v", err)
	}
	portB, err := runner.Output(ctx, "docker", "port", copyB, "80/tcp")
	if err != nil {
		t.Fatalf("docker port copyB failed: %v", err)
	}
	if portA == "" || portB == "" {
		t.Fatalf("published ports must not be empty: copyA=%q copyB=%q", portA, portB)
	}
	if portA == portB {
		t.Fatalf("published ports must differ to avoid host conflict: copyA=%q copyB=%q", portA, portB)
	}

	networkA, err := runner.Output(ctx, "docker", "inspect", "-f", "{{range $name, $_ := .NetworkSettings.Networks}}{{$name}}{{end}}", copyA)
	if err != nil {
		t.Fatalf("inspect copyA network failed: %v", err)
	}
	networkB, err := runner.Output(ctx, "docker", "inspect", "-f", "{{range $name, $_ := .NetworkSettings.Networks}}{{$name}}{{end}}", copyB)
	if err != nil {
		t.Fatalf("inspect copyB network failed: %v", err)
	}
	if strings.TrimSpace(networkA) != network || strings.TrimSpace(networkB) != network {
		t.Fatalf("containers must share network %q: copyA=%q copyB=%q", network, networkA, networkB)
	}
}

func TestDockerIntegrationは実MySQLのSchemaを専用Volumeへコピーする(t *testing.T) {
	if os.Getenv("PARACELL_RUN_DOCKER_TESTS") != "1" {
		t.Skip("set PARACELL_RUN_DOCKER_TESTS=1 to run Docker integration tests")
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skipf("Docker is not available: %v", err)
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	prefix := "paracell-integration-schema-" + suffix
	sourceContainer := prefix + "-source"
	targetContainer := prefix + "-db"
	sourceVolume := prefix + "-source-db"
	targetVolume := copiedVolumeName(targetContainer, "/var/lib/mysql")
	network := cellNetworkName(domain.Cell{Containers: domain.Containers{Services: map[string]domain.CellContainer{
		"db": {ContainerName: targetContainer},
	}}})

	t.Cleanup(func() {
		_ = exec.Command("docker", "rm", "-f", targetContainer, sourceContainer).Run()
		_ = exec.Command("docker", "network", "rm", network).Run()
		_ = exec.Command("docker", "volume", "rm", targetVolume, sourceVolume).Run()
	})

	runDockerIntegrationCommand(t, "volume", "create", sourceVolume)
	runDockerIntegrationCommand(t,
		"run", "-d", "--name", sourceContainer,
		"-e", "MYSQL_ROOT_PASSWORD=rootsecret",
		"-e", "MYSQL_DATABASE=myapp",
		"-e", "MYSQL_USER=app",
		"-e", "MYSQL_PASSWORD=secret",
		"-v", sourceVolume+":/var/lib/mysql",
		"mysql:8.0",
	)
	waitForIntegrationMySQL(t, sourceContainer)
	runDockerIntegrationCommand(t, "exec", sourceContainer, "mysql", "-uapp", "-psecret", "myapp", "-e",
		"CREATE TABLE users (id BIGINT PRIMARY KEY, name VARCHAR(255)); INSERT INTO users VALUES (1, 'source');")
	runDockerIntegrationCommand(t, "exec", sourceContainer, "mysql", "-uroot", "-prootsecret", "-e",
		"CREATE DATABASE analytics; CREATE TABLE analytics.events (id BIGINT PRIMARY KEY); INSERT INTO analytics.events VALUES (1);")

	cell := domain.Cell{
		Name: "integration",
		Containers: domain.Containers{
			NetworkMode: "isolated",
			Services: map[string]domain.CellContainer{
				"db": {
					ContainerName:   targetContainer,
					SourceContainer: sourceContainer,
					VolumeMode:      "copy",
					Database: &domain.DatabaseConfig{
						System:   "mysql",
						CopyMode: "schema",
					},
				},
			},
		},
	}
	template := domain.Template{Containers: domain.ContainerTemplate{Services: map[string]domain.ContainerServiceTemplate{
		"db": {SourceContainer: sourceContainer},
	}}}
	adapter := DockerCLIAdapter{Runner: system.CaptureRunner{}}
	if err := adapter.CreateContainers(context.Background(), cell, template); err != nil {
		t.Fatalf("CreateContainers with real MySQL failed: %v", err)
	}

	output := dockerIntegrationOutput(t, "exec", targetContainer, "mysql", "-uapp", "-psecret", "myapp", "-Nse",
		"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='myapp' AND table_name='users'; SELECT COUNT(*) FROM users;")
	if output != "1\n0" {
		t.Fatalf("target schema/table rows = %q, want %q", output, "1\n0")
	}
	analyticsOutput := dockerIntegrationOutput(t, "exec", targetContainer, "mysql", "-uroot", "-prootsecret", "-Nse",
		"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='analytics' AND table_name='events'; SELECT COUNT(*) FROM analytics.events;")
	if analyticsOutput != "1\n0" {
		t.Fatalf("second target schema/table rows = %q, want %q", analyticsOutput, "1\n0")
	}

	runDockerIntegrationCommand(t, "exec", targetContainer, "mysql", "-uapp", "-psecret", "myapp", "-e",
		"INSERT INTO users VALUES (2, 'cell');")
	targetRow := dockerIntegrationOutput(t, "exec", targetContainer, "mysql", "-uapp", "-psecret", "myapp", "-Nse",
		"SELECT CONCAT(id, ':', name) FROM users WHERE id=2;")
	if targetRow != "2:cell" {
		t.Fatalf("target row = %q, want %q", targetRow, "2:cell")
	}

	sourceRows := dockerIntegrationOutput(t, "exec", sourceContainer, "mysql", "-uapp", "-psecret", "myapp", "-Nse",
		"SELECT COUNT(*) FROM users WHERE id=2;")
	if sourceRows != "0" {
		t.Fatalf("source rows written by target = %q, want %q", sourceRows, "0")
	}
}

func waitForIntegrationMySQL(t *testing.T, container string) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		if exec.Command("docker", "exec", container, "mysqladmin", "ping", "-h", "127.0.0.1", "-uapp", "-psecret", "--silent").Run() == nil {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("MySQL container %q did not become ready", container)
}

func runDockerIntegrationCommand(t *testing.T, args ...string) {
	t.Helper()
	cmd := exec.Command("docker", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("docker %s failed: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
}

func dockerIntegrationOutput(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.Command("docker", args...)
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("docker %s failed: %v", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(output))
}
