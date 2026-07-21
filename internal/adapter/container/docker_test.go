package container

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hgsg11/paracell/internal/domain"
)

func TestDockerRun引数を組み立てられる(t *testing.T) {
	spec := RunSpec{
		Name:           "paracell-myapp-123-web",
		Image:          "nginx:latest",
		Network:        "paracell-myapp-123",
		NetworkAliases: []string{"web", "api"},
		Env:            []string{"APP_ENV=dev"},
		Entrypoint:     []string{"/docker-entrypoint.sh"},
		Command:        []string{"nginx", "-g", "daemon off;"},
		WorkDir:        "/app",
		User:           "node",
		Tty:            true,
		OpenStdin:      true,
		Health: HealthcheckSpec{
			Command:  "curl -f http://localhost:8080/health || exit 1",
			Interval: 30 * time.Second,
			Timeout:  5 * time.Second,
			Retries:  3,
		},
		Mounts:       []string{"/tmp/src:/app"},
		ExposedPorts: []string{"80"},
	}

	args := BuildDockerRunArgs(spec)

	want := []string{
		"run", "-d",
		"--name", "paracell-myapp-123-web",
		"--network", "paracell-myapp-123",
		"--network-alias", "web",
		"--network-alias", "api",
		"-e", "APP_ENV=dev",
		"--entrypoint", "/docker-entrypoint.sh",
		"-w", "/app",
		"--user", "node",
		"-t",
		"-i",
		"-v", "/tmp/src:/app",
		"-p", "80",
		"--health-cmd", "curl -f http://localhost:8080/health || exit 1",
		"--health-interval", "30s",
		"--health-timeout", "5s",
		"--health-retries", "3",
		"nginx:latest",
		"nginx", "-g", "daemon off;",
	}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("docker run args = %#v, want %#v", args, want)
	}
}

func TestBuildDockerRunArgsは内部Portだけを公開できる(t *testing.T) {
	spec := RunSpec{
		Name:         "paracell-myapp-123-web",
		Image:        "nginx:latest",
		ExposedPorts: []string{"3000"},
	}

	args := BuildDockerRunArgs(spec)

	want := []string{
		"run", "-d",
		"--name", "paracell-myapp-123-web",
		"-p", "3000",
		"nginx:latest",
	}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("docker run args = %#v, want %#v", args, want)
	}
}

func TestCreateContainersはSourceContainerの設定を復元して作成する(t *testing.T) {
	runner := &fakeRunner{
		outputs: []string{
			`{"Config":{"Image":"myapp-web:latest","Env":["APP_ENV=dev","PATH=/usr/bin"],"Entrypoint":["/docker-entrypoint.sh"],"Cmd":["npm","run","dev"],"WorkingDir":"/app","User":"node","Tty":true,"OpenStdin":true,"Healthcheck":{"Test":["CMD-SHELL","curl -f http://localhost:3000/health || exit 1"],"Interval":30000000000,"Timeout":5000000000,"Retries":3}},"HostConfig":{"PortBindings":{"3000/tcp":[{"HostIp":"","HostPort":"13000"}]}},"Mounts":[{"Type":"bind","Source":"/project","Destination":"/app","RW":true},{"Type":"bind","Source":"/project/config","Destination":"/config","RW":false},{"Type":"volume","Name":"myapp_vendor","Source":"/var/lib/docker/volumes/myapp_vendor/_data","Destination":"/app/vendor","RW":true}],"NetworkSettings":{"Networks":{"myapp_default":{}}}}`,
		},
	}
	adapter := DockerCLIAdapter{Runner: runner, Root: "/project"}
	cell := domain.Cell{
		Name: "123",
		Source: domain.Source{
			Path: "/project/.paracell/cells/123/source",
		},
		Containers: domain.Containers{
			NetworkMode: "isolated",
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
		"docker run -d --name paracell-myapp-123-web --network paracell-myapp-123 -e APP_ENV=dev -e PATH=/usr/bin --entrypoint /docker-entrypoint.sh -w /app --user node -t -i -v /project/.paracell/cells/123/source:/app -v /project/.paracell/cells/123/source/config:/config:ro -v myapp_vendor:/app/vendor:ro -p 3000 --health-cmd curl -f http://localhost:3000/health || exit 1 --health-interval 30s --health-timeout 5s --health-retries 3 myapp-web:latest npm run dev",
	}
	if !reflect.DeepEqual(runner.runCalls, wantRunCalls) {
		t.Fatalf("run calls = %#v, want %#v", runner.runCalls, wantRunCalls)
	}
}

func TestCreateContainersはIsolatedNetworkに元ContainerのAliasをコピーする(t *testing.T) {
	runner := &fakeRunner{
		outputs: []string{
			`{"Config":{"Image":"myapp-web:latest"},"Mounts":[],"NetworkSettings":{"Networks":{"myapp_default":{"Aliases":["myapp-web-1","web"]},"shared":{"Aliases":["web","frontend",""]}}}}`,
		},
	}
	adapter := DockerCLIAdapter{Runner: runner, Root: "/project"}
	cell := domain.Cell{
		Name: "123",
		Containers: domain.Containers{
			NetworkMode: "isolated",
			Services: map[string]domain.CellContainer{
				"web": {ContainerName: "paracell-myapp-123-web", SourceContainer: "myapp-web-1"},
			},
		},
	}
	template := domain.Template{Containers: domain.ContainerTemplate{Services: map[string]domain.ContainerServiceTemplate{
		"web": {SourceContainer: "myapp-web-1"},
	}}}

	if err := adapter.CreateContainers(context.Background(), cell, template); err != nil {
		t.Fatalf("CreateContainersでエラーが返った: %v", err)
	}

	wantRunCalls := []string{
		"docker network create paracell-myapp-123",
		"docker run -d --name paracell-myapp-123-web --network paracell-myapp-123 --network-alias frontend --network-alias myapp-web-1 --network-alias web myapp-web:latest",
	}
	if !reflect.DeepEqual(runner.runCalls, wantRunCalls) {
		t.Fatalf("run calls = %#v, want %#v", runner.runCalls, wantRunCalls)
	}
}

func TestCreateContainersはSharedNetworkで元ネットワークを使う(t *testing.T) {
	runner := &fakeRunner{
		outputs: []string{
			`{"Config":{"Image":"myapp-web:latest","Env":["APP_ENV=dev"]},"Mounts":[],"NetworkSettings":{"Networks":{"myapp_default":{}}}}`,
		},
	}
	adapter := DockerCLIAdapter{Runner: runner, Root: "/project"}
	cell := domain.Cell{
		Name:   "123",
		Source: domain.Source{Path: "/project/.paracell/cells/123/source"},
		Containers: domain.Containers{
			NetworkMode: "shared",
			Network:     "shared",
			Services: map[string]domain.CellContainer{
				"web": {ContainerName: "paracell-myapp-123-web", SourceContainer: "myapp-web"},
			},
		},
	}
	template := domain.Template{
		Containers: domain.ContainerTemplate{
			Network: "shared",
			Services: map[string]domain.ContainerServiceTemplate{
				"web": {SourceContainer: "myapp-web"},
			},
		},
	}

	if err := adapter.CreateContainers(context.Background(), cell, template); err != nil {
		t.Fatalf("CreateContainersでエラーが返った: %v", err)
	}

	wantRunCalls := []string{
		"docker run -d --name paracell-myapp-123-web --network myapp_default -e APP_ENV=dev myapp-web:latest",
	}
	if !reflect.DeepEqual(runner.runCalls, wantRunCalls) {
		t.Fatalf("run calls = %#v, want %#v", runner.runCalls, wantRunCalls)
	}
}

func TestCreateContainersはSharedNetworkで元コンテナの全Networkを使う(t *testing.T) {
	runner := &fakeRunner{
		outputs: []string{
			`{"Config":{"Image":"myapp-web:latest","Env":["APP_ENV=dev"]},"Mounts":[],"NetworkSettings":{"Networks":{"bridge":{},"myapp_default":{},"shared_front":{}}}}`,
		},
	}
	adapter := DockerCLIAdapter{Runner: runner, Root: "/project"}
	cell := domain.Cell{
		Name:   "123",
		Source: domain.Source{Path: "/project/.paracell/cells/123/source"},
		Containers: domain.Containers{
			NetworkMode: "shared",
			Network:     "shared",
			Services: map[string]domain.CellContainer{
				"web": {ContainerName: "paracell-myapp-123-web", SourceContainer: "myapp-web"},
			},
		},
	}
	template := domain.Template{
		Containers: domain.ContainerTemplate{
			Network: "shared",
			Services: map[string]domain.ContainerServiceTemplate{
				"web": {SourceContainer: "myapp-web"},
			},
		},
	}

	if err := adapter.CreateContainers(context.Background(), cell, template); err != nil {
		t.Fatalf("CreateContainersでエラーが返った: %v", err)
	}

	wantRunCalls := []string{
		"docker run -d --name paracell-myapp-123-web --network bridge -e APP_ENV=dev myapp-web:latest",
		"docker network connect myapp_default paracell-myapp-123-web",
		"docker network connect shared_front paracell-myapp-123-web",
	}
	if !reflect.DeepEqual(runner.runCalls, wantRunCalls) {
		t.Fatalf("run calls = %#v, want %#v", runner.runCalls, wantRunCalls)
	}
}

func TestCreateContainersは元Containerの内部Portだけをコピーする(t *testing.T) {
	runner := &fakeRunner{
		outputs: []string{
			`{"Config":{"Image":"myapp-web:latest","Env":["APP_ENV=dev"]},"HostConfig":{"PortBindings":{"3000/tcp":[{"HostIp":"","HostPort":"13000"}]}},"Mounts":[],"NetworkSettings":{"Networks":{"myapp_default":{}}}}`,
		},
	}
	adapter := DockerCLIAdapter{Runner: runner, Root: "/project"}
	cell := domain.Cell{
		Name:   "123",
		Source: domain.Source{Path: "/project/.paracell/cells/123/source"},
		Containers: domain.Containers{
			NetworkMode: "isolated",
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

	if len(runner.runCalls) != 2 {
		t.Fatalf("run calls length = %d, want 2 (%#v)", len(runner.runCalls), runner.runCalls)
	}
	if got := runner.runCalls[1]; strings.Contains(got, "-p 13000:3000") {
		t.Fatalf("run call = %q, want no copied host port binding", got)
	}
	if got := runner.runCalls[1]; !strings.Contains(got, " -p 3000 ") {
		t.Fatalf("run call = %q, want copied container port only", got)
	}
}

func TestCreateContainersはMySQLSchemaCopyを実行する(t *testing.T) {
	runner := &fakeRunner{
		outputs: []string{
			`{"Config":{"Image":"mysql:8","Env":["MYSQL_DATABASE=myapp","MYSQL_USER=app","MYSQL_PASSWORD=secret","MYSQL_ROOT_PASSWORD=rootsecret"]},"Mounts":[{"Type":"volume","Name":"myapp_db","Source":"/var/lib/docker/volumes/myapp_db/_data","Destination":"/var/lib/mysql","RW":true}],"NetworkSettings":{"Networks":{"myapp_default":{}}}}`,
			"analytics\nmyapp\n",
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
			NetworkMode: "isolated",
			Services: map[string]domain.CellContainer{
				"db": {
					ContainerName:   "paracell-myapp-123-db",
					SourceContainer: "myapp-db",
					VolumeMode:      "copy",
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
		`docker exec myapp-db mysql --batch --skip-column-names -u root -prootsecret -e SELECT SCHEMA_NAME FROM INFORMATION_SCHEMA.SCHEMATA WHERE SCHEMA_NAME NOT IN ('information_schema', 'mysql', 'performance_schema', 'sys') ORDER BY SCHEMA_NAME`,
		`docker exec myapp-db mysqldump --no-data --no-tablespaces -u root -prootsecret --databases analytics myapp`,
	}
	if !reflect.DeepEqual(runner.outputCalls, wantOutputCalls) {
		t.Fatalf("output calls = %#v, want %#v", runner.outputCalls, wantOutputCalls)
	}
	if len(runner.runCalls) != 7 {
		t.Fatalf("run calls length = %d, want 7 (%#v)", len(runner.runCalls), runner.runCalls)
	}
	if got := runner.runCalls[0]; got != "docker network create paracell-myapp-123" {
		t.Fatalf("network create call = %q", got)
	}
	if got := runner.runCalls[1]; got != "docker volume create paracell-myapp-123-db-var-lib-mysql" {
		t.Fatalf("volume create call = %q", got)
	}
	if got := runner.runCalls[2]; got != "docker run -d --name paracell-myapp-123-db --network paracell-myapp-123 -e MYSQL_DATABASE=myapp -e MYSQL_USER=app -e MYSQL_PASSWORD=secret -e MYSQL_ROOT_PASSWORD=rootsecret -v paracell-myapp-123-db-var-lib-mysql:/var/lib/mysql:rw mysql:8" {
		t.Fatalf("first run call = %q", got)
	}
	if got := runner.runCalls[3]; got != "docker exec paracell-myapp-123-db mysqladmin ping -h 127.0.0.1 -u root -prootsecret --silent" {
		t.Fatalf("wait run call = %q", got)
	}
	if got := runner.runCalls[4]; !strings.HasPrefix(got, "docker cp ") || !strings.HasSuffix(got, " paracell-myapp-123-db:/tmp/paracell-schema.sql") {
		t.Fatalf("cp run call = %q", got)
	}
	if got := runner.runCalls[5]; got != "docker exec paracell-myapp-123-db sh -c mysql -u 'root' '-prootsecret' < '/tmp/paracell-schema.sql'" {
		t.Fatalf("import run call = %q", got)
	}
	if got := runner.runCalls[6]; got != "docker exec paracell-myapp-123-db rm -f /tmp/paracell-schema.sql" {
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
			NetworkMode: "isolated",
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
			NetworkMode: "isolated",
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

func TestCopiedVolumeNameは不正文字を安全な文字へ変換する(t *testing.T) {
	got := copiedVolumeName(`paracell-myapp-feature\volume-web`, `/app/vendor cache`)
	want := "paracell-myapp-feature-volume-web-app-vendor-cache"
	if got != want {
		t.Fatalf("copiedVolumeName() = %q, want %q", got, want)
	}
}

func TestCleanContainersはコンテナ削除後にセルネットワークを削除する(t *testing.T) {
	runner := &fakeRunner{}
	adapter := DockerCLIAdapter{Runner: runner}
	cell := domain.Cell{
		Containers: domain.Containers{
			NetworkMode: "isolated",
			Network:     "paracell-myapp-123",
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

func TestCleanContainersは見つからないContainerとNetworkを無視する(t *testing.T) {
	runner := &fakeRunner{
		runErrors: map[string]error{
			"docker rm -f paracell-myapp-123-web":  errors.New("Error response from daemon: No such container: paracell-myapp-123-web"),
			"docker network rm paracell-myapp-123": errors.New("Error response from daemon: network paracell-myapp-123 not found"),
		},
	}
	adapter := DockerCLIAdapter{Runner: runner}
	cell := domain.Cell{
		Containers: domain.Containers{
			NetworkMode: "isolated",
			Network:     "paracell-myapp-123",
			Services: map[string]domain.CellContainer{
				"web": {ContainerName: "paracell-myapp-123-web"},
			},
		},
	}

	err := adapter.CleanContainers(context.Background(), cell)
	if !errors.Is(err, domain.ErrNotFound) && err != nil {
		t.Fatalf("error = %v, want nil or domain.ErrNotFound-consumed behavior", err)
	}
}

type fakeRunner struct {
	outputs     []string
	outputCalls []string
	runCalls    []string
	runErrors   map[string]error
}

func (r *fakeRunner) Run(ctx context.Context, name string, args ...string) error {
	_ = ctx
	call := name + " " + joinArgs(args)
	r.runCalls = append(r.runCalls, call)
	if r.runErrors != nil && r.runErrors[call] != nil {
		return r.runErrors[call]
	}
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
