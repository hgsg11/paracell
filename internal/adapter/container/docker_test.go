package container

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/hgsg11/paracell/internal/domain"
)

func TestCreateContainersはTargetを作りDependencyを接続する(t *testing.T) {
	runner := &fakeRunner{outputs: []string{
		`{"Config":{"Image":"app:latest"},"Mounts":[],"NetworkSettings":{"Networks":{"default":{"Aliases":["app"]}}}}`,
		`{"Config":{"Image":"db:latest"},"Mounts":[],"NetworkSettings":{"Networks":{"default":{"Aliases":["db"]}}}}`,
	}}
	environment, err := domain.NewEnvironment("A", "B")
	if err != nil {
		t.Fatal(err)
	}
	app, err := domain.NewContainerTemplate("app", domain.Target, []domain.Environment{environment}, nil)
	if err != nil {
		t.Fatal(err)
	}
	db, err := domain.NewContainerTemplate("db", domain.Dependency, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	templates := []domain.ContainerTemplate{app, db}
	if _, err := (DockerCLIAdapter{Runner: runner, Root: "/project"}).CreateContainers(context.Background(), templates, "42", "sample", "cell-42", ".paracell/cells/42/source"); err != nil {
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

func TestCreateContainersはTemplateMountと既存MountをCellへ適用する(t *testing.T) {
	runner := &fakeRunner{outputs: []string{
		`{"Config":{"Image":"app:latest"},"Mounts":[{"Type":"volume","Name":"original-data","Destination":"/data","RW":true},{"Type":"bind","Source":"/project/src","Destination":"/app","RW":false}],"NetworkSettings":{"Networks":{}}}`,
	}}
	mount, err := domain.NewMount("/extra", "extras")
	if err != nil {
		t.Fatal(err)
	}
	template, err := domain.NewContainerTemplate("app", domain.Target, nil, []domain.Mount{mount})
	if err != nil {
		t.Fatal(err)
	}
	adapter := NewDockerCLIAdapter(runner, "/project")
	_, err = adapter.CreateContainers(context.Background(), []domain.ContainerTemplate{template}, "42", "sample", "cell-42", ".paracell/cells/42/source")
	if err != nil {
		t.Fatal(err)
	}
	calls := strings.Join(runner.runCalls, "\n")
	for _, want := range []string{
		"docker volume create cell-42-app-data",
		"-v original-data:/from:ro -v cell-42-app-data:/to",
		"-v cell-42-app-data:/data",
		"-v /project/.paracell/cells/42/source/src:/app:ro",
		"-v /project/.paracell/cells/42/source/extras:/extra",
	} {
		if !strings.Contains(calls, want) {
			t.Errorf("missing %q in %s", want, calls)
		}
	}
}

func TestCreateContainersは後続失敗時にDependencyを切断する(t *testing.T) {
	failure := errors.New("inspect failed")
	runner := &fakeRunner{
		outputs:      []string{`{"NetworkSettings":{"Networks":{"original":{"Aliases":["db"]}}}}`, ""},
		outputErrors: []error{nil, failure},
	}
	dependency, err := domain.NewContainerTemplate("db", domain.Dependency, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	target, err := domain.NewContainerTemplate("web", domain.Target, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewDockerCLIAdapter(runner, "/project").CreateContainers(context.Background(), []domain.ContainerTemplate{target, dependency}, "42", "sample", "cell-42", "")
	if !errors.Is(err, failure) {
		t.Fatalf("error = %v", err)
	}
	calls := strings.Join(runner.runCalls, "\n")
	for _, want := range []string{"docker network connect --alias db cell-42 db", "docker network disconnect cell-42 db", "docker network rm cell-42"} {
		if !strings.Contains(calls, want) {
			t.Errorf("missing %q in %s", want, calls)
		}
	}
	if strings.Contains(calls, "docker rm -f db") {
		t.Fatalf("dependency removed: %s", calls)
	}
}

func TestCleanContainersはTargetを削除しDependencyを切断してErrorを集約する(t *testing.T) {
	failure := errors.New("remove failed")
	runner := &fakeRunner{runErrors: map[string]error{
		"docker rm -f cell-42-app":             failure,
		"docker network disconnect cell-42 db": errors.New("not connected"),
	}}
	err := NewDockerCLIAdapter(runner, "/project").CleanContainers(context.Background(), "cell-42", []string{"cell-42-app"}, []string{"db"})
	if !errors.Is(err, failure) {
		t.Fatalf("error = %v", err)
	}
	want := []string{"docker rm -f cell-42-app", "docker network disconnect cell-42 db", "docker network disconnect -f cell-42 paracell-gateway", "docker network rm cell-42"}
	if !reflect.DeepEqual(runner.runCalls, want) {
		t.Fatalf("calls = %v", runner.runCalls)
	}
}
