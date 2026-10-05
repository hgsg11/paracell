package container

import (
	"context"
	"strings"
	"testing"

	"github.com/hgsg11/paracell/internal/domain"
)

func TestCreateContainersはTargetを作りDependencyを接続する(t *testing.T) {
	runner := &fakeRunner{outputs: []string{
		`{"Config":{"Image":"app:latest"},"Mounts":[],"NetworkSettings":{"Networks":{"default":{"Aliases":["app"]}}}}`,
		`db-container`,
		`{"Config":{"Image":"db:latest"},"Mounts":[],"NetworkSettings":{"Networks":{"default":{"Aliases":["db"]}}}}`,
	}}
	resources := domain.NewContainerResources("42", "sample", "cell-42", []domain.ContainerResource{
		func() domain.ContainerResource {
			r := domain.NewContainerResource("cell-42-app", nil, "app", domain.Target, []domain.Environment{{Name: "A", Value: "B"}}, nil)
			r.SourcePath = ".paracell/cells/42/source"
			return r
		}(),
		domain.NewContainerResource("db", nil, "db", domain.Dependency, nil, nil),
	})
	networks, err := (DockerCLIAdapter{Runner: runner, Root: "/project"}).CreateContainers(context.Background(), resources)
	if err != nil {
		t.Fatal(err)
	}
	calls := strings.Join(runner.runCalls, "\n")
	if !strings.Contains(calls, "docker run -d --name cell-42-app") || !strings.Contains(calls, "docker network connect --alias db cell-42 db") {
		t.Fatalf("calls = %s", calls)
	}
	if !strings.Contains(strings.Join(runner.outputCalls, "\n"), "docker ps --filter label=com.docker.compose.service=db") {
		t.Fatalf("dependency was not resolved from its Compose service: %v", runner.outputCalls)
	}
	if !contains(networks["db"], "cell-42") {
		t.Fatalf("dependency networks = %v, want issue-specific network cell-42", networks["db"])
	}
}

func contains(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

func TestCopiedVolumeNameは同一ServiceとPathで共有する(t *testing.T) {
	first := copiedVolumeName("group-a", "database", "/var/lib/data")
	second := copiedVolumeName("group-a", "database", "/var/lib/data")
	if first != second {
		t.Fatalf("volume names differ: %q / %q", first, second)
	}
	if first == copiedVolumeName("group-a", "cache", "/var/lib/data") || first == copiedVolumeName("group-a", "database", "/var/lib/cache") || first == copiedVolumeName("group-b", "database", "/var/lib/data") {
		t.Fatal("volume was shared across a different Source or path")
	}
}

func TestCopyMountsは同じSourcePathのVolumeを共有する(t *testing.T) {
	adapter := DockerCLIAdapter{Runner: &fakeRunner{}}
	mount := dockerMount{Type: "volume", Name: "compose-data", Destination: "/var/lib/data", RW: true}
	first := domain.NewContainerResource("cell-a-api", nil, "database", domain.Target, nil, nil)
	second := domain.NewContainerResource("cell-a-worker", nil, "database", domain.Target, nil, nil)
	firstMounts, err := adapter.copyMounts(context.Background(), "cell-network", "", first, []dockerMount{mount}, nil)
	if err != nil {
		t.Fatal(err)
	}
	secondMounts, err := adapter.copyMounts(context.Background(), "cell-network", "", second, []dockerMount{mount}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(firstMounts) != 1 || len(secondMounts) != 1 || firstMounts[0] != secondMounts[0] {
		t.Fatalf("shared volume mounts = %v / %v", firstMounts, secondMounts)
	}
}

type fakeRunner struct {
	outputs              []string
	outputErrors         []error
	outputCalls          []string
	runCalls             []string
	runErrors            map[string]error
	gatewayInspectOutput *string
	gatewayInspectError  error
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) error {
	call := strings.Join(append([]string{name}, args...), " ")
	f.runCalls = append(f.runCalls, call)
	return f.runErrors[call]
}

func (f *fakeRunner) Output(_ context.Context, name string, args ...string) (string, error) {
	call := strings.Join(append([]string{name}, args...), " ")
	f.outputCalls = append(f.outputCalls, call)
	if name == "docker" && len(args) > 0 && args[len(args)-1] == gatewayContainerName {
		if f.gatewayInspectOutput != nil || f.gatewayInspectError != nil {
			if f.gatewayInspectOutput == nil {
				return "", f.gatewayInspectError
			}
			return *f.gatewayInspectOutput, f.gatewayInspectError
		}
		network := ""
		for i := len(f.runCalls) - 1; i >= 0; i-- {
			if strings.HasPrefix(f.runCalls[i], "docker network create ") {
				network = strings.TrimPrefix(f.runCalls[i], "docker network create ")
				break
			}
		}
		return `{"Config":{"Labels":{"io.paracell.gateway":"true","io.paracell.gateway.config-version":"3"}},"State":{"Running":true},"NetworkSettings":{"Networks":{"` + network + `":{}}}}`, nil
	}
	if len(args) > 0 && args[0] == "ps" {
		output := f.outputs[0]
		f.outputs = f.outputs[1:]
		return output, nil
	}
	output := f.outputs[0]
	f.outputs = f.outputs[1:]
	if len(f.outputErrors) == 0 {
		return output, nil
	}
	err := f.outputErrors[0]
	f.outputErrors = f.outputErrors[1:]
	return output, err
}
