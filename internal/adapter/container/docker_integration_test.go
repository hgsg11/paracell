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

func TestDockerIntegrationは同じSourceComposeから作った2CellのFrontendBackendURLを分離する(t *testing.T) {
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
	sourceNetwork := "paracell-integrationgw-source-" + suffix
	sourceContainers := map[string]string{
		"frontend": "integrationgw-source-frontend-" + suffix,
		"backend":  "integrationgw-source-backend-" + suffix,
	}
	adapter := DockerCLIAdapter{Runner: system.CaptureRunner{}}
	cells := make([]domain.Cell, 0, 2)
	for _, issue := range []string{"issue-a-" + suffix, "issue-b-" + suffix} {
		network := "paracell-integrationgw-" + issue
		cells = append(cells, domain.Cell{
			Name: issue,
			Containers: domain.Containers{
				Network: network,
				Services: map[string]domain.CellContainer{
					"frontend": {ContainerName: network + "-frontend", SourceContainer: sourceContainers["frontend"]},
					"backend":  {ContainerName: network + "-backend", SourceContainer: sourceContainers["backend"]},
				},
			},
		})
	}
	template := domain.Template{Containers: domain.ContainerTemplate{Services: map[string]domain.ContainerServiceTemplate{
		"frontend": {SourceContainer: sourceContainers["frontend"]},
		"backend":  {SourceContainer: sourceContainers["backend"]},
	}}}

	t.Cleanup(func() {
		for _, cell := range cells {
			_ = adapter.CleanContainers(context.Background(), cell)
		}
		for _, sourceContainer := range sourceContainers {
			_ = exec.Command("docker", "rm", "-f", sourceContainer).Run()
		}
		_ = exec.Command("docker", "network", "rm", sourceNetwork).Run()
		if removeGateway {
			_ = exec.Command("docker", "rm", "-f", gatewayContainerName).Run()
		}
	})

	runDockerIntegrationCommand(t, "network", "create", sourceNetwork)
	for role, sourceContainer := range sourceContainers {
		runDockerIntegrationCommand(t,
			"run", "-d", "--name", sourceContainer,
			"--network", sourceNetwork,
			"--network-alias", role,
			"--network-alias", "compose-generated-"+role,
			"-p", "80",
			"nginx:alpine",
		)
	}
	for _, cell := range cells {
		if err := adapter.CreateContainers(context.Background(), cell, template); err != nil {
			t.Fatalf("CreateContainers for cell %q failed: %v", cell.Name, err)
		}
	}
	gatewayEndpoint := strings.Split(dockerIntegrationOutput(t, "port", gatewayContainerName, "80/tcp"), "\n")[0]

	for _, cell := range cells {
		for _, role := range []string{"frontend", "backend"} {
			host := role + "." + cell.Name + ".integrationgw.localhost"
			waitForGatewayHost(t, gatewayEndpoint, host)
		}
	}
	verifyGatewayDashboard(t, gatewayEndpoint)

	for _, cell := range cells {
		for role, service := range cell.Containers.Services {
			networks := dockerIntegrationOutput(t, "inspect", "-f", "{{range $name, $_ := .NetworkSettings.Networks}}{{$name}}{{end}}", service.ContainerName)
			if networks != cell.Containers.Network {
				t.Fatalf("%s container networks = %q, want %q", role, networks, cell.Containers.Network)
			}
			labels := dockerIntegrationOutput(t, "inspect", "-f", "{{json .Config.Labels}}", service.ContainerName)
			if strings.Contains(labels, "compose-generated-") || strings.Contains(labels, sourceContainers[role]+".") {
				t.Fatalf("%s labels contain a source alias Host rule: %s", service.ContainerName, labels)
			}
		}
	}
}

func waitForGatewayHost(t *testing.T, gatewayEndpoint string, host string) {
	t.Helper()
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
				return
			}
			lastErr = fmt.Errorf("status=%s body=%q readErr=%v", response.Status, strings.TrimSpace(string(body)), readErr)
		} else {
			lastErr = err
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("gateway did not route host %q: %v", host, lastErr)
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

func TestDockerIntegrationはCell専用NetworkへAliasをコピーする(t *testing.T) {
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

func TestDockerIntegrationは同一MySQLを2Cellで共有して個別Cleanできる(t *testing.T) {
	if os.Getenv("PARACELL_RUN_DOCKER_TESTS") != "1" {
		t.Skip("set PARACELL_RUN_DOCKER_TESTS=1 to run Docker integration tests")
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skipf("Docker is not available: %v", err)
	}
	for _, image := range []string{"mysql:8.0", gatewayImage} {
		if err := exec.Command("docker", "image", "inspect", image).Run(); err != nil {
			t.Skipf("%s image is required: %v", image, err)
		}
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	prefix := "paracell-integration-shared-db-" + suffix
	sourceNetwork := prefix + "-source"
	databaseContainer := prefix + "-mysql"
	applicationSource := prefix + "-app-source"
	adapter := DockerCLIAdapter{Runner: system.CaptureRunner{}}
	cells := make([]domain.Cell, 0, 2)
	for _, name := range []string{"a", "b"} {
		network := prefix + "-cell-" + name
		cells = append(cells, domain.Cell{Name: name, Containers: domain.Containers{
			Network: network,
			Services: map[string]domain.CellContainer{
				"db": {
					ContainerName: network + "-db", SourceContainer: databaseContainer,
					Database: &domain.DatabaseConfig{Mode: domain.DatabaseModeShared},
				},
				"app": {ContainerName: network + "-app", SourceContainer: applicationSource},
			},
		}})
	}
	template := domain.Template{Containers: domain.ContainerTemplate{Services: map[string]domain.ContainerServiceTemplate{
		"db":  {SourceContainer: databaseContainer, Database: &domain.DatabaseConfig{Mode: domain.DatabaseModeShared}},
		"app": {SourceContainer: applicationSource},
	}}}

	t.Cleanup(func() {
		for _, cell := range cells {
			_ = adapter.CleanContainers(context.Background(), cell)
		}
		_ = exec.Command("docker", "rm", "-f", applicationSource, databaseContainer).Run()
		_ = exec.Command("docker", "network", "rm", sourceNetwork).Run()
	})

	runDockerIntegrationCommand(t, "network", "create", sourceNetwork)
	runDockerIntegrationCommand(t,
		"run", "-d", "--name", databaseContainer,
		"--network", sourceNetwork, "--network-alias", "primary-db",
		"-e", "MYSQL_ROOT_PASSWORD=rootsecret", "-e", "MYSQL_DATABASE=myapp",
		"-e", "MYSQL_USER=app", "-e", "MYSQL_PASSWORD=secret",
		"mysql:8.0",
	)
	waitForIntegrationMySQL(t, databaseContainer)
	runDockerIntegrationCommand(t,
		"run", "-d", "--name", applicationSource,
		"--network", sourceNetwork, "--network-alias", "source-app",
		"--entrypoint", "sh", "mysql:8.0", "-c", "sleep 300",
	)

	for _, cell := range cells {
		if err := adapter.CreateContainers(context.Background(), cell, template); err != nil {
			t.Fatalf("CreateContainers for cell %q failed: %v", cell.Name, err)
		}
		app := cell.Containers.Services["app"].ContainerName
		output := dockerIntegrationOutput(t, "exec", app, "mysql", "-h", "primary-db", "-uapp", "-psecret", "myapp", "-Nse", "SELECT 1")
		if output != "1" {
			t.Fatalf("shared database query from cell %q = %q, want 1", cell.Name, output)
		}
		networksJSON := dockerIntegrationOutput(t, "inspect", "-f", "{{json .NetworkSettings.Networks}}", databaseContainer)
		var attached map[string]dockerNetwork
		if err := json.Unmarshal([]byte(networksJSON), &attached); err != nil {
			t.Fatalf("decode shared database aliases for cell %q: %v", cell.Name, err)
		}
		aliases := attached[cell.Containers.Network].Aliases
		if !containsString(aliases, "primary-db") || containsString(aliases, "db") {
			t.Fatalf("shared database aliases for cell %q = %#v, want primary-db without fixed db", cell.Name, aliases)
		}
	}

	if err := adapter.CleanContainers(context.Background(), cells[0]); err != nil {
		t.Fatalf("clean first cell: %v", err)
	}
	networks := dockerIntegrationOutput(t, "inspect", "-f", "{{json .NetworkSettings.Networks}}", databaseContainer)
	if strings.Contains(networks, cells[0].Containers.Network) || !strings.Contains(networks, cells[1].Containers.Network) || !strings.Contains(networks, sourceNetwork) {
		t.Fatalf("database networks after first clean = %s", networks)
	}
	appB := cells[1].Containers.Services["app"].ContainerName
	if output := dockerIntegrationOutput(t, "exec", appB, "mysql", "-h", "primary-db", "-uapp", "-psecret", "myapp", "-Nse", "SELECT 1"); output != "1" {
		t.Fatalf("second cell lost shared database after first clean: %q", output)
	}
}

func TestCreateContainersは別CellNetworkで同一ContainerPortを競合せず公開できる(t *testing.T) {
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
	sourceNetwork := "paracell-port-conflict-test-source-net"
	cellNetworkA := "paracell-port-conflict-test-a-net"
	cellNetworkB := "paracell-port-conflict-test-b-net"
	sourceContainer := "paracell-port-conflict-source"
	copyA := cellNetworkA + "-web"
	copyB := cellNetworkB + "-web"

	cleanup := []string{copyA, copyB, sourceContainer}
	for _, name := range cleanup {
		defer exec.Command("docker", "rm", "-f", name).Run()
	}
	defer exec.Command("docker", "network", "rm", sourceNetwork).Run()
	defer exec.Command("docker", "network", "rm", cellNetworkA).Run()
	defer exec.Command("docker", "network", "rm", cellNetworkB).Run()

	_ = exec.Command("docker", "rm", "-f", sourceContainer).Run()
	_ = exec.Command("docker", "rm", "-f", copyA).Run()
	_ = exec.Command("docker", "rm", "-f", copyB).Run()
	_ = exec.Command("docker", "network", "rm", sourceNetwork).Run()
	_ = exec.Command("docker", "network", "rm", cellNetworkA).Run()
	_ = exec.Command("docker", "network", "rm", cellNetworkB).Run()

	if err := runner.Run(ctx, "docker", "network", "create", sourceNetwork); err != nil {
		t.Fatalf("network create failed: %v", err)
	}
	if err := runner.Run(ctx, "docker", "run", "-d", "--name", sourceContainer, "--network", sourceNetwork, "-p", "80", "nginx:alpine"); err != nil {
		t.Fatalf("source container run failed: %v", err)
	}

	adapter := DockerCLIAdapter{Runner: runner}
	template := domain.Template{
		Containers: domain.ContainerTemplate{
			Services: map[string]domain.ContainerServiceTemplate{
				"web": {SourceContainer: sourceContainer},
			},
		},
	}

	cellA := domain.Cell{
		Name: "copy-a",
		Containers: domain.Containers{
			Network: cellNetworkA,
			Services: map[string]domain.CellContainer{
				"web": {ContainerName: copyA, SourceContainer: sourceContainer},
			},
		},
	}
	cellB := domain.Cell{
		Name: "copy-b",
		Containers: domain.Containers{
			Network: cellNetworkB,
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

	actualNetworkA, err := runner.Output(ctx, "docker", "inspect", "-f", "{{range $name, $_ := .NetworkSettings.Networks}}{{$name}}{{end}}", copyA)
	if err != nil {
		t.Fatalf("inspect copyA network failed: %v", err)
	}
	actualNetworkB, err := runner.Output(ctx, "docker", "inspect", "-f", "{{range $name, $_ := .NetworkSettings.Networks}}{{$name}}{{end}}", copyB)
	if err != nil {
		t.Fatalf("inspect copyB network failed: %v", err)
	}
	if strings.TrimSpace(actualNetworkA) != cellNetworkA || strings.TrimSpace(actualNetworkB) != cellNetworkB {
		t.Fatalf("containers must use separate cell networks: copyA=%q copyB=%q", actualNetworkA, actualNetworkB)
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
