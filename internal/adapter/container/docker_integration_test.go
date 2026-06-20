package container

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/hgsg11/paracell/internal/adapter/system"
	"github.com/hgsg11/paracell/internal/domain"
)

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
