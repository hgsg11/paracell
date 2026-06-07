package container

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/hgsg11/paracell/internal/domain"
)

func TestDockerRun引数を組み立てられる(t *testing.T) {
	spec := RunSpec{
		Name:       "paracell-myapp-123-web",
		Image:      "nginx:latest",
		Network:    "paracell-myapp-123",
		Env:        []string{"APP_ENV=dev"},
		Entrypoint: []string{"/docker-entrypoint.sh"},
		Command:    []string{"nginx", "-g", "daemon off;"},
		WorkDir:    "/app",
		Mounts:     []string{"/tmp/src:/app"},
		Ports:      map[string]string{"8080": "80"},
	}

	args := BuildDockerRunArgs(spec)

	want := []string{
		"run", "-d",
		"--name", "paracell-myapp-123-web",
		"--network", "paracell-myapp-123",
		"-e", "APP_ENV=dev",
		"--entrypoint", "/docker-entrypoint.sh",
		"-w", "/app",
		"-v", "/tmp/src:/app",
		"-p", "8080:80",
		"nginx:latest",
		"nginx", "-g", "daemon off;",
	}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("docker run args = %#v, want %#v", args, want)
	}
}

func TestCreateContainersはSourceContainerの設定を復元して作成する(t *testing.T) {
	runner := &fakeRunner{
		outputs: []string{
			`{"Config":{"Image":"myapp-web:latest","Env":["APP_ENV=dev","PATH=/usr/bin"]},"Mounts":[{"Type":"bind","Source":"/project","Destination":"/app","RW":true},{"Type":"bind","Source":"/project/config","Destination":"/config","RW":false},{"Type":"volume","Name":"myapp_vendor","Source":"/var/lib/docker/volumes/myapp_vendor/_data","Destination":"/app/vendor","RW":true}],"NetworkSettings":{"Networks":{"myapp_default":{}}}}`,
		},
	}
	adapter := DockerCLIAdapter{Runner: runner, Root: "/project"}
	cell := domain.Cell{
		Name: "123",
		Source: domain.Source{
			Path: "/project/.paracell/cells/123/source",
		},
		Containers: domain.Containers{
			Services: map[string]domain.CellContainer{
				"web": {ContainerName: "paracell-myapp-123-web", SourceContainer: "myapp-web"},
			},
		},
	}
	template := domain.Template{
		Containers: domain.ContainerTemplate{
			Services: map[string]domain.ContainerServiceTemplate{
				"web": {SourceContainer: "myapp-web"},
			},
		},
	}

	if err := adapter.CreateContainers(context.Background(), cell, template); err != nil {
		t.Fatalf("CreateContainersでエラーが返った: %v", err)
	}

	wantOutputCalls := []string{`docker inspect -f {{json .}} myapp-web`}
	if !reflect.DeepEqual(runner.outputCalls, wantOutputCalls) {
		t.Fatalf("output calls = %#v, want %#v", runner.outputCalls, wantOutputCalls)
	}
	wantRunCalls := []string{
		"docker network create paracell-myapp-123",
		"docker run -d --name paracell-myapp-123-web --network paracell-myapp-123 -e APP_ENV=dev -e PATH=/usr/bin -v /project/.paracell/cells/123/source:/app -v /project/.paracell/cells/123/source/config:/config:ro -v myapp_vendor:/app/vendor:ro myapp-web:latest",
	}
	if !reflect.DeepEqual(runner.runCalls, wantRunCalls) {
		t.Fatalf("run calls = %#v, want %#v", runner.runCalls, wantRunCalls)
	}
}

func TestCreateContainersはMySQLSchemaCopyを実行する(t *testing.T) {
	runner := &fakeRunner{
		outputs: []string{
			`{"Config":{"Image":"mysql:8","Env":["MYSQL_DATABASE=myapp","MYSQL_USER=app","MYSQL_PASSWORD=secret"]},"Mounts":[],"NetworkSettings":{"Networks":{"myapp_default":{}}}}`,
			"CREATE TABLE users (id bigint primary key);",
		},
	}
	adapter := DockerCLIAdapter{Runner: runner, Root: "/project"}
	cell := domain.Cell{
		Name: "123",
		Source: domain.Source{
			Path: "/project/.paracell/cells/123/source",
		},
		Containers: domain.Containers{
			Services: map[string]domain.CellContainer{
				"db": {
					ContainerName:   "paracell-myapp-123-db",
					SourceContainer: "myapp-db",
					Database: &domain.DatabaseConfig{
						System:   "mysql",
						CopyMode: "schema",
					},
				},
			},
		},
	}
	template := domain.Template{
		Containers: domain.ContainerTemplate{
			Services: map[string]domain.ContainerServiceTemplate{
				"db": {SourceContainer: "myapp-db"},
			},
		},
	}

	if err := adapter.CreateContainers(context.Background(), cell, template); err != nil {
		t.Fatalf("CreateContainersでエラーが返った: %v", err)
	}

	wantOutputCalls := []string{
		`docker inspect -f {{json .}} myapp-db`,
		`docker exec myapp-db mysqldump --no-data -u app -psecret myapp`,
	}
	if !reflect.DeepEqual(runner.outputCalls, wantOutputCalls) {
		t.Fatalf("output calls = %#v, want %#v", runner.outputCalls, wantOutputCalls)
	}
	if len(runner.runCalls) != 6 {
		t.Fatalf("run calls length = %d, want 6 (%#v)", len(runner.runCalls), runner.runCalls)
	}
	if got := runner.runCalls[0]; got != "docker network create paracell-myapp-123" {
		t.Fatalf("network create call = %q", got)
	}
	if got := runner.runCalls[1]; got != "docker run -d --name paracell-myapp-123-db --network paracell-myapp-123 -e MYSQL_DATABASE=myapp -e MYSQL_USER=app -e MYSQL_PASSWORD=secret mysql:8" {
		t.Fatalf("first run call = %q", got)
	}
	if got := runner.runCalls[2]; got != "docker exec paracell-myapp-123-db mysqladmin ping -h 127.0.0.1 -u app -psecret --silent" {
		t.Fatalf("wait run call = %q", got)
	}
	if got := runner.runCalls[3]; !strings.HasPrefix(got, "docker cp ") || !strings.HasSuffix(got, " paracell-myapp-123-db:/tmp/paracell-schema.sql") {
		t.Fatalf("cp run call = %q", got)
	}
	if got := runner.runCalls[4]; got != "docker exec paracell-myapp-123-db sh -c mysql -u 'app' '-psecret' 'myapp' < '/tmp/paracell-schema.sql'" {
		t.Fatalf("import run call = %q", got)
	}
	if got := runner.runCalls[5]; got != "docker exec paracell-myapp-123-db rm -f /tmp/paracell-schema.sql" {
		t.Fatalf("cleanup run call = %q", got)
	}
}

func TestCreateContainersはDataCopyを未実装エラーにする(t *testing.T) {
	runner := &fakeRunner{
		outputs: []string{
			`{"Config":{"Image":"mysql:8","Env":["MYSQL_DATABASE=myapp","MYSQL_USER=app","MYSQL_PASSWORD=secret"]},"Mounts":[],"NetworkSettings":{"Networks":{"myapp_default":{}}}}`,
		},
	}
	adapter := DockerCLIAdapter{Runner: runner, Root: "/project"}
	cell := domain.Cell{
		Containers: domain.Containers{
			Services: map[string]domain.CellContainer{
				"db": {
					ContainerName:   "paracell-myapp-123-db",
					SourceContainer: "myapp-db",
					Database: &domain.DatabaseConfig{
						System:   "mysql",
						CopyMode: "data",
					},
				},
			},
		},
	}
	template := domain.Template{
		Containers: domain.ContainerTemplate{
			Services: map[string]domain.ContainerServiceTemplate{
				"db": {SourceContainer: "myapp-db"},
			},
		},
	}

	err := adapter.CreateContainers(context.Background(), cell, template)
	if err == nil {
		t.Fatal("data copyなのにエラーが返らなかった")
	}
	if err.Error() != `copyMode "data" is not implemented for service "db"` {
		t.Fatalf("error = %q, want %q", err.Error(), `copyMode "data" is not implemented for service "db"`)
	}
}

func TestCreateContainersはInitFilesをCellSource経由でMountする(t *testing.T) {
	runner := &fakeRunner{
		outputs: []string{
			`{"Config":{"Image":"mysql:8","Env":["MYSQL_DATABASE=myapp","MYSQL_USER=app","MYSQL_PASSWORD=secret"]},"Mounts":[],"NetworkSettings":{"Networks":{"myapp_default":{}}}}`,
		},
	}
	adapter := DockerCLIAdapter{Runner: runner, Root: "/project"}
	cell := domain.Cell{
		Name: "123",
		Source: domain.Source{
			Path: "/project/.paracell/cells/123/source",
		},
		Containers: domain.Containers{
			Services: map[string]domain.CellContainer{
				"db": {
					ContainerName:   "paracell-myapp-123-db",
					SourceContainer: "myapp-db",
					Database: &domain.DatabaseConfig{
						System:    "mysql",
						InitFiles: []string{"docker/mysql/init/001-users.sql"},
					},
				},
			},
		},
	}
	template := domain.Template{
		Containers: domain.ContainerTemplate{
			Services: map[string]domain.ContainerServiceTemplate{
				"db": {SourceContainer: "myapp-db"},
			},
		},
	}

	if err := adapter.CreateContainers(context.Background(), cell, template); err != nil {
		t.Fatalf("CreateContainersでエラーが返った: %v", err)
	}

	if len(runner.runCalls) != 2 {
		t.Fatalf("run calls length = %d, want 2", len(runner.runCalls))
	}
	if got := runner.runCalls[0]; got != "docker network create paracell-myapp-123" {
		t.Fatalf("network create call = %q", got)
	}
	got := runner.runCalls[1]
	wantMount := "-v /project/.paracell/cells/123/source/docker/mysql/init/001-users.sql:/docker-entrypoint-initdb.d/001-users.sql:ro"
	if !strings.Contains(got, wantMount) {
		t.Fatalf("run call = %q, want init mount %q", got, wantMount)
	}
}

func TestCreateContainersはVolumeModeCopyでNamedVolumeを複製する(t *testing.T) {
	runner := &fakeRunner{
		outputs: []string{
			`{"Config":{"Image":"myapp-web:latest","Env":["APP_ENV=dev"]},"Mounts":[{"Type":"volume","Name":"myapp_vendor","Source":"/var/lib/docker/volumes/myapp_vendor/_data","Destination":"/app/vendor","RW":true}],"NetworkSettings":{"Networks":{"myapp_default":{}}}}`,
		},
	}
	adapter := DockerCLIAdapter{Runner: runner, Root: "/project"}
	cell := domain.Cell{
		Name: "123",
		Containers: domain.Containers{
			Services: map[string]domain.CellContainer{
				"web": {
					ContainerName:   "paracell-myapp-123-web",
					SourceContainer: "myapp-web",
					VolumeMode:      "copy",
				},
			},
		},
	}
	template := domain.Template{
		Containers: domain.ContainerTemplate{
			Services: map[string]domain.ContainerServiceTemplate{
				"web": {SourceContainer: "myapp-web"},
			},
		},
	}

	if err := adapter.CreateContainers(context.Background(), cell, template); err != nil {
		t.Fatalf("CreateContainersでエラーが返った: %v", err)
	}

	wantRunCalls := []string{
		"docker network create paracell-myapp-123",
		"docker volume create paracell-myapp-123-web-app-vendor",
		"docker run --rm -v myapp_vendor:/from:ro -v paracell-myapp-123-web-app-vendor:/to alpine sh -c cp -a /from/. /to/",
		"docker run -d --name paracell-myapp-123-web --network paracell-myapp-123 -e APP_ENV=dev -v paracell-myapp-123-web-app-vendor:/app/vendor myapp-web:latest",
	}
	if !reflect.DeepEqual(runner.runCalls, wantRunCalls) {
		t.Fatalf("run calls = %#v, want %#v", runner.runCalls, wantRunCalls)
	}
}

func TestCleanContainersはコンテナ削除後にセルネットワークを削除する(t *testing.T) {
	runner := &fakeRunner{}
	adapter := DockerCLIAdapter{Runner: runner}
	cell := domain.Cell{
		Containers: domain.Containers{
			Network: "paracell-myapp-123",
			Services: map[string]domain.CellContainer{
				"web": {ContainerName: "paracell-myapp-123-web"},
				"db":  {ContainerName: "paracell-myapp-123-db"},
			},
		},
	}

	if err := adapter.CleanContainers(context.Background(), cell); err != nil {
		t.Fatalf("CleanContainersでエラーが返った: %v", err)
	}

	want := []string{
		"docker rm -f paracell-myapp-123-db",
		"docker rm -f paracell-myapp-123-web",
		"docker network rm paracell-myapp-123",
	}
	if !reflect.DeepEqual(runner.runCalls, want) {
		t.Fatalf("run calls = %#v, want %#v", runner.runCalls, want)
	}
}

type fakeRunner struct {
	outputs     []string
	outputCalls []string
	runCalls    []string
}

func (r *fakeRunner) Run(ctx context.Context, name string, args ...string) error {
	_ = ctx
	r.runCalls = append(r.runCalls, name+" "+joinArgs(args))
	return nil
}

func (r *fakeRunner) Output(ctx context.Context, name string, args ...string) (string, error) {
	_ = ctx
	r.outputCalls = append(r.outputCalls, name+" "+joinArgs(args))
	out := r.outputs[0]
	r.outputs = r.outputs[1:]
	return out, nil
}

func joinArgs(args []string) string {
	if len(args) == 0 {
		return ""
	}
	out := args[0]
	for _, arg := range args[1:] {
		out += " " + arg
	}
	return out
}
