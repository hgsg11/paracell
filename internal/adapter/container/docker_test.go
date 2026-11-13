package container

import (
	"context"
	"errors"
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
	prepared := map[string]struct{}{}
	firstMounts, err := adapter.copyMounts(context.Background(), "cell-network", "", first, []dockerMount{mount}, nil, prepared)
	if err != nil {
		t.Fatal(err)
	}
	secondMounts, err := adapter.copyMounts(context.Background(), "cell-network", "", second, []dockerMount{mount}, nil, prepared)
	if err != nil {
		t.Fatal(err)
	}
	if len(firstMounts) != 1 || len(secondMounts) != 1 || firstMounts[0] != secondMounts[0] {
		t.Fatalf("shared volume mounts = %v / %v", firstMounts, secondMounts)
	}
	copies := 0
	for _, call := range adapter.Runner.(*fakeRunner).runCalls {
		if strings.Contains(call, "cp -a /from/. /to/") {
			copies++
		}
	}
	if copies != 1 {
		t.Fatalf("shared volume was copied %d times, want once", copies)
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

func TestCreateContainersはDocker資源名を小文字に変換する(t *testing.T) {
	runner := &fakeRunner{outputs: []string{
		`{"Config":{"Image":"app:latest"},"Mounts":[],"NetworkSettings":{"Networks":{"default":{"Aliases":["app"]}}}}`,
		`db-container`,
		`{"Config":{"Image":"db:latest"},"Mounts":[],"NetworkSettings":{"Networks":{"default":{"Aliases":["db"]}}}}`,
	}}
	resources := domain.NewContainerResources("ABC-42", "Sample", "paracell-Sample-ABC-42", []domain.ContainerResource{
		domain.NewContainerResource("paracell-Sample-ABC-42-Api-App", nil, "App", domain.Target, nil, nil),
		domain.NewContainerResource("DB", nil, "DB", domain.Dependency, nil, nil),
	})
	networks, err := (DockerCLIAdapter{Runner: runner, Root: "/project"}).CreateContainers(context.Background(), resources)
	if err != nil {
		t.Fatal(err)
	}
	calls := strings.Join(runner.runCalls, "\n")
	for _, want := range []string{
		"docker network create paracell-sample-abc-42",
		"docker run -d --name paracell-sample-abc-42-api-app --network paracell-sample-abc-42",
		"--label com.docker.compose.project=paracell-sample-abc-42",
		"docker network connect --alias db paracell-sample-abc-42 db-container",
	} {
		if !strings.Contains(calls, want) {
			t.Fatalf("calls missing %q:\n%s", want, calls)
		}
	}
	if strings.Contains(calls, "ABC") || strings.Contains(calls, "Sample") {
		t.Fatalf("calls contain uppercase resource names:\n%s", calls)
	}
	if !strings.Contains(strings.Join(runner.outputCalls, "\n"), "docker ps --filter label=com.docker.compose.service=DB") {
		t.Fatalf("dependency service name must be kept as-is: %v", runner.outputCalls)
	}
	if !contains(networks["DB"], "paracell-sample-abc-42") {
		t.Fatalf("dependency networks = %v", networks["DB"])
	}
}

func TestCleanContainersはDocker資源名を小文字に変換する(t *testing.T) {
	runner := &fakeRunner{gatewayInspectError: errors.New("No such container: paracell-gateway"), gatewayInspectOutput: new(string)}
	resources := domain.NewContainerResources("ABC-42", "Sample", "paracell-Sample-ABC-42", []domain.ContainerResource{
		domain.NewContainerResource("paracell-Sample-ABC-42-Api-App", nil, "App", domain.Target, nil, nil),
	})
	if err := (DockerCLIAdapter{Runner: runner}).CleanContainers(context.Background(), resources); err != nil {
		t.Fatal(err)
	}
	calls := strings.Join(runner.runCalls, "\n")
	if !strings.Contains(calls, "docker rm -f paracell-sample-abc-42-api-app") || !strings.Contains(calls, "docker network rm paracell-sample-abc-42") {
		t.Fatalf("calls = %s", calls)
	}
	if resources.Network != "paracell-Sample-ABC-42" || resources.Items[0].Name != "paracell-Sample-ABC-42-Api-App" {
		t.Fatal("input resources must not be modified")
	}
}

func TestCopiedVolumeNameは小文字を返す(t *testing.T) {
	if got := copiedVolumeName("paracell-Sample-ABC-42", "DB", "/var/Lib"); got != "paracell-sample-abc-42-db-var-lib" {
		t.Fatalf("volume name = %q", got)
	}
}

func TestCreateContainersは同じServiceを使うターゲットを区別する(t *testing.T) {
	inspect := `{"Config":{"Image":"app:latest"},"HostConfig":{"PortBindings":{"8080/tcp":[{"HostPort":"8080"}]}},"Mounts":[{"Type":"volume","Name":"compose-data","Destination":"/data","RW":true}],"NetworkSettings":{"Networks":{"default":{"Aliases":["app"]}}}}`
	runner := &fakeRunner{outputs: []string{inspect, inspect}}
	api := domain.NewContainerResource("paracell-Sample-42-API-app", nil, "app", domain.Target, []domain.Environment{{Name: "ROLE", Value: "api"}}, nil)
	api.Target, api.Role = "API", "app.API"
	worker := domain.NewContainerResource("paracell-Sample-42-worker-app", nil, "app", domain.Target, []domain.Environment{{Name: "ROLE", Value: "worker"}}, nil)
	worker.Target, worker.Role = "worker", "app.worker"
	resources := domain.NewContainerResources("42", "Sample", "paracell-Sample-42", []domain.ContainerResource{worker, api})
	networks, err := (DockerCLIAdapter{Runner: runner, Root: "/project"}).CreateContainers(context.Background(), resources)
	if err != nil {
		t.Fatal(err)
	}
	calls := strings.Join(runner.runCalls, "\n")
	for _, want := range []string{
		"--name paracell-sample-42-api-app",
		"-e ROLE=api",
		"Host(`app.api.42.sample.localhost`)",
		"--name paracell-sample-42-worker-app",
		"-e ROLE=worker",
		"Host(`app.worker.42.sample.localhost`)",
		"--network-alias app",
	} {
		if !strings.Contains(calls, want) {
			t.Fatalf("calls missing %q:\n%s", want, calls)
		}
	}
	if strings.Contains(calls, "--network-alias app.") {
		t.Fatalf("network aliases must not be qualified:\n%s", calls)
	}
	if got := strings.Count(calls, "cp -a /from/. /to/"); got != 1 {
		t.Fatalf("shared volume copied %d times:\n%s", got, calls)
	}
	for _, name := range []string{"paracell-Sample-42-API-app", "paracell-Sample-42-worker-app"} {
		if _, ok := networks[name]; !ok {
			t.Fatalf("networks are not keyed by resource name: %v", networks)
		}
	}
}

func TestCreateContainersはFilesを元のMountより優先する(t *testing.T) {
	runner := &fakeRunner{outputs: []string{
		`{"Config":{"Image":"app:latest"},"Mounts":[{"Type":"volume","Name":"compose-data","Destination":"/data/","RW":true},{"Type":"volume","Name":"compose-cache","Destination":"/cache","RW":true}],"NetworkSettings":{"Networks":{"default":{"Aliases":["app"]}}}}`,
	}}
	mount, err := domain.NewMount("/data", "fixtures")
	if err != nil {
		t.Fatal(err)
	}
	app := domain.NewContainerResource("cell-42-app", nil, "app", domain.Target, nil, []domain.Mount{mount})
	app.SourcePath = ".paracell/cells/42/api/source"
	if _, err := (DockerCLIAdapter{Runner: runner, Root: "/project"}).CreateContainers(context.Background(), domain.NewContainerResources("42", "sample", "cell-42", []domain.ContainerResource{app})); err != nil {
		t.Fatal(err)
	}
	calls := strings.Join(runner.runCalls, "\n")
	if !strings.Contains(calls, "-v /project/.paracell/cells/42/api/source/fixtures:/data") || !strings.Contains(calls, ":/cache") {
		t.Fatalf("calls = %s", calls)
	}
	if strings.Contains(calls, "compose-data") || strings.Count(calls, ":/data") != 1 {
		t.Fatalf("overridden mount was still used:\n%s", calls)
	}
}
