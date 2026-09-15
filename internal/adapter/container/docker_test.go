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
		`{"Config":{"Image":"db:latest"},"Mounts":[],"NetworkSettings":{"Networks":{"default":{"Aliases":["db"]}}}}`,
	}}
	resources := domain.NewContainerResources("42", "sample", "cell-42", ".paracell/cells/42/source", []domain.ContainerResource{
		domain.NewContainerResource("app", "cell-42-app", nil, "app", domain.Target, []domain.Environment{{Name: "A", Value: "B"}}, nil),
		domain.NewContainerResource("db", "db", nil, "db", domain.Dependency, nil, nil),
	})
	if _, err := (DockerCLIAdapter{Runner: runner, Root: "/project"}).CreateContainers(context.Background(), resources); err != nil {
		t.Fatal(err)
	}
	calls := strings.Join(runner.runCalls, "\n")
	if !strings.Contains(calls, "docker run -d --name cell-42-app") || !strings.Contains(calls, "docker network connect --alias db cell-42 db") {
		t.Fatalf("calls = %s", calls)
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
	output := f.outputs[0]
	f.outputs = f.outputs[1:]
	if len(f.outputErrors) == 0 {
		return output, nil
	}
	err := f.outputErrors[0]
	f.outputErrors = f.outputErrors[1:]
	return output, err
}
