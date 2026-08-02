package container

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/hgsg11/paracell/internal/domain"
)

func TestGatewayLabelsは単一PortをAliasCellProjectのHostへRouteする(t *testing.T) {
	cell := gatewayTestCell()

	labels := gatewayLabels(cell, "paracell-myapp-123-web", []string{"web", "frontend"}, map[string][]dockerPortBinding{
		"3000/tcp": {{HostPort: "13000"}},
	})

	name := "paracell-myapp-123-web-p3000"
	want := map[string]string{
		"traefik.enable":                                              "true",
		"traefik.docker.network":                                      "paracell-myapp-123",
		"traefik.http.routers." + name + ".entrypoints":               "web",
		"traefik.http.routers." + name + ".rule":                      "Host(`frontend.123.myapp.localhost`) || Host(`web.123.myapp.localhost`)",
		"traefik.http.routers." + name + ".service":                   name,
		"traefik.http.services." + name + ".loadbalancer.server.port": "3000",
	}
	if !reflect.DeepEqual(labels, want) {
		t.Fatalf("gateway labels = %#v, want %#v", labels, want)
	}
}

func TestGatewayLabelsは複数PortへPortPrefix付きHostを作る(t *testing.T) {
	cell := gatewayTestCell()

	labels := gatewayLabels(cell, "paracell-myapp-123-web", []string{"web"}, map[string][]dockerPortBinding{
		"8080/tcp": nil,
		"3000/tcp": nil,
		"53/udp":   nil,
	})

	if got := labels["traefik.http.routers.paracell-myapp-123-web-p3000.rule"]; got != "Host(`p3000.web.123.myapp.localhost`)" {
		t.Fatalf("3000 route = %q", got)
	}
	if got := labels["traefik.http.routers.paracell-myapp-123-web-p8080.rule"]; got != "Host(`p8080.web.123.myapp.localhost`)" {
		t.Fatalf("8080 route = %q", got)
	}
	if _, exists := labels["traefik.http.services.paracell-myapp-123-web-p53.loadbalancer.server.port"]; exists {
		t.Fatal("UDP port must not produce an HTTP gateway route")
	}
}

func TestEnsureGatewayはLoopbackにGatewayを作りCellNetworkへ接続する(t *testing.T) {
	empty := ""
	runner := &fakeRunner{
		gatewayInspectOutput: &empty,
		gatewayInspectError:  errors.New("No such container: paracell-gateway"),
	}
	adapter := DockerCLIAdapter{Runner: runner}

	if err := adapter.ensureGateway(context.Background(), "paracell-myapp-123"); err != nil {
		t.Fatalf("ensureGateway returned error: %v", err)
	}

	want := []string{
		"docker run -d --name paracell-gateway --label io.paracell.gateway=true --restart unless-stopped -p 127.0.0.1:80:80 -v /var/run/docker.sock:/var/run/docker.sock:ro traefik:v3.7 --providers.docker=true --providers.docker.exposedbydefault=false --entrypoints.web.address=:80",
		"docker network connect paracell-myapp-123 paracell-gateway",
	}
	if !reflect.DeepEqual(runner.runCalls, want) {
		t.Fatalf("run calls = %#v, want %#v", runner.runCalls, want)
	}
}

func TestEnsureGatewayは実行中かつ接続済みのGatewayを再利用する(t *testing.T) {
	output := `{"Config":{"Labels":{"io.paracell.gateway":"true"}},"State":{"Running":true},"NetworkSettings":{"Networks":{"paracell-myapp-123":{}}}}`
	runner := &fakeRunner{gatewayInspectOutput: &output}
	adapter := DockerCLIAdapter{Runner: runner}

	if err := adapter.ensureGateway(context.Background(), "paracell-myapp-123"); err != nil {
		t.Fatalf("ensureGateway returned error: %v", err)
	}
	if len(runner.runCalls) != 0 {
		t.Fatalf("run calls = %#v, want none", runner.runCalls)
	}
}

func TestEnsureGatewayは既存Gatewayを別CellNetworkへ接続する(t *testing.T) {
	output := `{"Config":{"Labels":{"io.paracell.gateway":"true"}},"State":{"Running":true},"NetworkSettings":{"Networks":{"paracell-myapp-122":{}}}}`
	runner := &fakeRunner{gatewayInspectOutput: &output}
	adapter := DockerCLIAdapter{Runner: runner}

	if err := adapter.ensureGateway(context.Background(), "paracell-myapp-123"); err != nil {
		t.Fatalf("ensureGateway returned error: %v", err)
	}
	want := []string{"docker network connect paracell-myapp-123 paracell-gateway"}
	if !reflect.DeepEqual(runner.runCalls, want) {
		t.Fatalf("run calls = %#v, want %#v", runner.runCalls, want)
	}
}

func TestEnsureGatewayは同名の非管理Containerを変更しない(t *testing.T) {
	output := `{"Config":{"Image":"nginx:alpine"},"State":{"Running":false},"NetworkSettings":{"Networks":{}}}`
	runner := &fakeRunner{gatewayInspectOutput: &output}
	adapter := DockerCLIAdapter{Runner: runner}

	err := adapter.ensureGateway(context.Background(), "paracell-myapp-123")
	if err == nil || !strings.Contains(err.Error(), "not managed by Paracell") {
		t.Fatalf("error = %v, want ownership error", err)
	}
	if len(runner.runCalls) != 0 {
		t.Fatalf("run calls = %#v, want none", runner.runCalls)
	}
}

func TestCreateContainersはIsolatedContainerへGatewayRouteを登録する(t *testing.T) {
	empty := ""
	runner := &fakeRunner{
		gatewayInspectOutput: &empty,
		gatewayInspectError:  errors.New("No such container: paracell-gateway"),
		outputs: []string{
			`{"Config":{"Image":"myapp-web:latest"},"HostConfig":{"PortBindings":{"3000/tcp":[{"HostPort":"13000"}]}},"Mounts":[],"NetworkSettings":{"Networks":{"myapp_default":{"Aliases":["web"]}}}}`,
		},
	}
	adapter := DockerCLIAdapter{Runner: runner, Root: "/project"}
	cell := gatewayTestCell()
	template := domain.Template{Containers: domain.ContainerTemplate{Services: map[string]domain.ContainerServiceTemplate{
		"web": {SourceContainer: "myapp-web"},
	}}}

	if err := adapter.CreateContainers(context.Background(), cell, template); err != nil {
		t.Fatalf("CreateContainers returned error: %v", err)
	}
	if len(runner.runCalls) != 4 {
		t.Fatalf("run calls = %#v, want network, gateway, connect, and application container", runner.runCalls)
	}
	applicationRun := runner.runCalls[3]
	for _, want := range []string{
		"--label traefik.docker.network=paracell-myapp-123",
		"--label traefik.enable=true",
		"--label traefik.http.routers.paracell-myapp-123-web-p3000.rule=Host(`web.123.myapp.localhost`)",
		"--label traefik.http.services.paracell-myapp-123-web-p3000.loadbalancer.server.port=3000",
	} {
		if !strings.Contains(applicationRun, want) {
			t.Fatalf("application run = %q, want fragment %q", applicationRun, want)
		}
	}
}

func TestCreateContainersはAliasやPortがなくてもIsolatedGatewayを接続する(t *testing.T) {
	empty := ""
	runner := &fakeRunner{
		gatewayInspectOutput: &empty,
		gatewayInspectError:  errors.New("No such container: paracell-gateway"),
		outputs: []string{
			`{"Config":{"Image":"myapp-worker:latest"},"Mounts":[],"NetworkSettings":{"Networks":{"myapp_default":{}}}}`,
		},
	}
	adapter := DockerCLIAdapter{Runner: runner}
	cell := gatewayTestCell()
	template := domain.Template{Containers: domain.ContainerTemplate{Services: map[string]domain.ContainerServiceTemplate{
		"web": {SourceContainer: "myapp-worker"},
	}}}

	if err := adapter.CreateContainers(context.Background(), cell, template); err != nil {
		t.Fatalf("CreateContainers returned error: %v", err)
	}
	if got := runner.runCalls[2]; got != "docker network connect paracell-myapp-123 paracell-gateway" {
		t.Fatalf("gateway connect = %q", got)
	}
	if strings.Contains(runner.runCalls[3], "traefik.") {
		t.Fatalf("container without alias/port has gateway labels: %q", runner.runCalls[3])
	}
}

func TestCreateContainersはGatewayのPort競合時にCellNetworkをRollbackする(t *testing.T) {
	empty := ""
	gatewayRun := "docker run -d --name paracell-gateway --label io.paracell.gateway=true --restart unless-stopped -p 127.0.0.1:80:80 -v /var/run/docker.sock:/var/run/docker.sock:ro traefik:v3.7 --providers.docker=true --providers.docker.exposedbydefault=false --entrypoints.web.address=:80"
	runner := &fakeRunner{
		gatewayInspectOutput: &empty,
		gatewayInspectError:  errors.New("No such container: paracell-gateway"),
		runErrors: map[string]error{
			gatewayRun: errors.New("Bind for 127.0.0.1:80 failed: port is already allocated"),
		},
	}
	adapter := DockerCLIAdapter{Runner: runner}

	err := adapter.CreateContainers(context.Background(), gatewayTestCell(), domain.Template{})
	if err == nil || !strings.Contains(err.Error(), "start Paracell gateway on 127.0.0.1:80") {
		t.Fatalf("error = %v, want actionable port conflict", err)
	}
	wantTail := []string{
		"docker network disconnect -f paracell-myapp-123 paracell-gateway",
		"docker network rm paracell-myapp-123",
	}
	if !reflect.DeepEqual(runner.runCalls[len(runner.runCalls)-2:], wantTail) {
		t.Fatalf("rollback calls = %#v, want %#v", runner.runCalls, wantTail)
	}
}

func TestCreateContainersは途中失敗時に作成済みContainerとNetworkをRollbackする(t *testing.T) {
	runner := &fakeRunner{
		outputs: []string{
			`{"Config":{"Image":"myapp-db:latest"},"Mounts":[],"NetworkSettings":{"Networks":{"myapp_default":{}}}}`,
			`{"Config":{"Image":"myapp-web:latest"},"Mounts":[],"NetworkSettings":{"Networks":{"myapp_default":{}}}}`,
		},
		runErrors: map[string]error{
			"docker run -d --name paracell-myapp-123-web --network paracell-myapp-123 --label com.docker.compose.project=paracell-myapp-123 --label com.docker.compose.service=web myapp-web:latest": errors.New("container start failed"),
		},
	}
	adapter := DockerCLIAdapter{Runner: runner}
	cell := gatewayTestCell()
	cell.Containers.Services["db"] = domain.CellContainer{ContainerName: "paracell-myapp-123-db", SourceContainer: "myapp-db"}
	template := domain.Template{Containers: domain.ContainerTemplate{Services: map[string]domain.ContainerServiceTemplate{
		"db":  {SourceContainer: "myapp-db"},
		"web": {SourceContainer: "myapp-web"},
	}}}

	err := adapter.CreateContainers(context.Background(), cell, template)
	if err == nil || !strings.Contains(err.Error(), "container start failed") {
		t.Fatalf("error = %v", err)
	}
	wantTail := []string{
		"docker rm -f paracell-myapp-123-db",
		"docker network disconnect -f paracell-myapp-123 paracell-gateway",
		"docker network rm paracell-myapp-123",
	}
	if !reflect.DeepEqual(runner.runCalls[len(runner.runCalls)-3:], wantTail) {
		t.Fatalf("rollback calls = %#v, want tail %#v", runner.runCalls, wantTail)
	}
}

func gatewayTestCell() domain.Cell {
	return domain.Cell{
		Name: "123",
		Containers: domain.Containers{
			Network:     "paracell-myapp-123",
			NetworkMode: "isolated",
			Services: map[string]domain.CellContainer{
				"web": {ContainerName: "paracell-myapp-123-web"},
			},
		},
	}
}
