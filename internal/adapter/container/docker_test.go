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
		Labels: map[string]string{
			"com.docker.compose.service": "web",
			"com.docker.compose.project": "paracell-myapp-123",
		},
		Env:        []string{"APP_ENV=dev"},
		Entrypoint: []string{"/docker-entrypoint.sh"},
		Command:    []string{"nginx", "-g", "daemon off;"},
		WorkDir:    "/app",
		User:       "node",
		Tty:        true,
		OpenStdin:  true,
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
		"--label", "com.docker.compose.project=paracell-myapp-123",
		"--label", "com.docker.compose.service=web",
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

func TestCreateContainersはIsolatedの複数ContainerをCell単位でグループ化する(t *testing.T) {
	runner := &fakeRunner{
		outputs: []string{
			`{"Config":{"Image":"myapp-db:latest"},"Mounts":[],"NetworkSettings":{"Networks":{"myapp_default":{}}}}`,
			`{"Config":{"Image":"myapp-web:latest"},"Mounts":[],"NetworkSettings":{"Networks":{"myapp_default":{}}}}`,
		},
	}
	adapter := DockerCLIAdapter{Runner: runner, Root: "/project"}
	cell := domain.Cell{
		Name: "123",
		Containers: domain.Containers{
			NetworkMode: "isolated",
			Network:     "paracell-myapp-123",
			Services: map[string]domain.CellContainer{
				"web": {ContainerName: "paracell-myapp-123-web", SourceContainer: "myapp-web"},
				"db":  {ContainerName: "paracell-myapp-123-db", SourceContainer: "myapp-db"},
			},
		},
	}
	template := domain.Template{Containers: domain.ContainerTemplate{Services: map[string]domain.ContainerServiceTemplate{
		"web": {SourceContainer: "myapp-web"},
		"db":  {SourceContainer: "myapp-db"},
	}}}

	if err := adapter.CreateContainers(context.Background(), cell, template); err != nil {
		t.Fatalf("CreateContainersでエラーが返った: %v", err)
	}

	wantRunCalls := []string{
		"docker network create paracell-myapp-123",
		"docker run -d --name paracell-myapp-123-db --network paracell-myapp-123 --network-alias db --label com.docker.compose.project=paracell-myapp-123 --label com.docker.compose.service=db myapp-db:latest",
		"docker run -d --name paracell-myapp-123-web --network paracell-myapp-123 --network-alias web --label com.docker.compose.project=paracell-myapp-123 --label com.docker.compose.service=web myapp-web:latest",
	}
	if !reflect.DeepEqual(runner.runCalls, wantRunCalls) {
		t.Fatalf("run calls = %#v, want %#v", runner.runCalls, wantRunCalls)
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
		"docker run -d --name paracell-myapp-123-web --network paracell-myapp-123 --network-alias web --label com.docker.compose.project=paracell-myapp-123 --label com.docker.compose.service=web --label traefik.docker.network=paracell-myapp-123 --label traefik.enable=true --label traefik.http.routers.paracell-myapp-123-web-p3000.entrypoints=web --label traefik.http.routers.paracell-myapp-123-web-p3000.rule=Host(`web.123.myapp.localhost`) --label traefik.http.routers.paracell-myapp-123-web-p3000.service=paracell-myapp-123-web-p3000 --label traefik.http.services.paracell-myapp-123-web-p3000.loadbalancer.server.port=3000 -e APP_ENV=dev -e PATH=/usr/bin --entrypoint /docker-entrypoint.sh -w /app --user node -t -i -v /project/.paracell/cells/123/source:/app -v /project/.paracell/cells/123/source/config:/config:ro -v myapp_vendor:/app/vendor:ro -p 3000 --health-cmd curl -f http://localhost:3000/health || exit 1 --health-interval 30s --health-timeout 5s --health-retries 3 myapp-web:latest npm run dev",
	}
	if !reflect.DeepEqual(runner.runCalls, wantRunCalls) {
		t.Fatalf("run calls = %#v, want %#v", runner.runCalls, wantRunCalls)
	}
}

func TestCreateContainersはSourceEnvironmentをService設定で上書きする(t *testing.T) {
	runner := &fakeRunner{
		outputs: []string{
			`{"Config":{"Image":"myapp-web:latest","Env":["APP_ENV=source","PATH=/usr/bin","EMPTY=source"]},"Mounts":[],"NetworkSettings":{"Networks":{"myapp_default":{}}}}`,
		},
	}
	adapter := DockerCLIAdapter{Runner: runner, Root: "/project"}
	cell := domain.Cell{
		Name: "123",
		Containers: domain.Containers{
			NetworkMode: "isolated",
			Services: map[string]domain.CellContainer{
				"web": {ContainerName: "paracell-myapp-123-web", SourceContainer: "myapp-web"},
			},
		},
	}
	template := domain.Template{Containers: domain.ContainerTemplate{Services: map[string]domain.ContainerServiceTemplate{
		"web": {
			SourceContainer: "myapp-web",
			Environment: map[string]string{
				"APP_ENV": "cell",
				"EMPTY":   "",
				"NEW_VAR": "new",
			},
		},
	}}}

	if err := adapter.CreateContainers(context.Background(), cell, template); err != nil {
		t.Fatalf("CreateContainersでエラーが返った: %v", err)
	}

	wantRunCalls := []string{
		"docker network create paracell-myapp-123",
		"docker run -d --name paracell-myapp-123-web --network paracell-myapp-123 --network-alias web --label com.docker.compose.project=paracell-myapp-123 --label com.docker.compose.service=web -e APP_ENV=cell -e PATH=/usr/bin -e EMPTY= -e NEW_VAR=new myapp-web:latest",
	}
	if !reflect.DeepEqual(runner.runCalls, wantRunCalls) {
		t.Fatalf("run calls = %#v, want %#v", runner.runCalls, wantRunCalls)
	}
}

func TestMergeEnvironmentは設定がなければSourceを保持する(t *testing.T) {
	source := []string{"APP_ENV=source", "PATH=/usr/bin"}
	got := mergeEnvironment(source, nil)

	if !reflect.DeepEqual(got, source) {
		t.Fatalf("merged environment = %#v, want %#v", got, source)
	}
	got[0] = "changed"
	if source[0] != "APP_ENV=source" {
		t.Fatalf("source environment was mutated: %#v", source)
	}
}

func TestCreateContainersはIsolatedNetworkに元ContainerのAliasをコピーする(t *testing.T) {
	runner := &fakeRunner{
		outputs: []string{
			`{"Config":{"Image":"myapp-web:latest"},"Mounts":[],"NetworkSettings":{"Networks":{"myapp_default":{"Aliases":["myapp-web-1"]},"shared":{"Aliases":["frontend",""]}}}}`,
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
		"docker run -d --name paracell-myapp-123-web --network paracell-myapp-123 --network-alias frontend --network-alias myapp-web-1 --network-alias web --label com.docker.compose.project=paracell-myapp-123 --label com.docker.compose.service=web myapp-web:latest",
	}
	if !reflect.DeepEqual(runner.runCalls, wantRunCalls) {
		t.Fatalf("run calls = %#v, want %#v", runner.runCalls, wantRunCalls)
	}
}

func TestCreateContainersはSharedDatabaseを既存AliasだけでCellNetworkへ接続する(t *testing.T) {
	runner := &fakeRunner{outputs: []string{
		`{"Config":{"Image":"mysql:8"},"Mounts":[{"Type":"volume","Name":"myapp_db","Destination":"/var/lib/mysql","RW":true}],"NetworkSettings":{"Networks":{"backend":{"Aliases":["mysql","myapp-db-1",""]}}}}`,
		`{"Config":{"Image":"myapp-web:latest"},"Mounts":[],"NetworkSettings":{"Networks":{"backend":{"Aliases":["web"]}}}}`,
	}}
	adapter := DockerCLIAdapter{Runner: runner, Root: "/project"}
	cell := domain.Cell{Name: "123", Containers: domain.Containers{
		NetworkMode: "isolated",
		Network:     "paracell-myapp-123",
		Services: map[string]domain.CellContainer{
			"db": {
				ContainerName:   "paracell-myapp-123-db",
				SourceContainer: "myapp-db-1",
				Database:        &domain.DatabaseConfig{Mode: domain.DatabaseModeShared},
			},
			"web": {ContainerName: "paracell-myapp-123-web", SourceContainer: "myapp-web"},
		},
	}}
	template := domain.Template{Containers: domain.ContainerTemplate{Services: map[string]domain.ContainerServiceTemplate{
		"db":  {SourceContainer: "myapp-db-1", Database: &domain.DatabaseConfig{Mode: domain.DatabaseModeShared}},
		"web": {SourceContainer: "myapp-web"},
	}}}

	if err := adapter.CreateContainers(context.Background(), cell, template); err != nil {
		t.Fatalf("CreateContainersでエラーが返った: %v", err)
	}

	want := []string{
		"docker network create paracell-myapp-123",
		"docker network connect --alias myapp-db-1 --alias mysql paracell-myapp-123 myapp-db-1",
		"docker run -d --name paracell-myapp-123-web --network paracell-myapp-123 --network-alias web --label com.docker.compose.project=paracell-myapp-123 --label com.docker.compose.service=web myapp-web:latest",
	}
	if !reflect.DeepEqual(runner.runCalls, want) {
		t.Fatalf("run calls = %#v, want %#v", runner.runCalls, want)
	}
	for _, call := range runner.runCalls {
		if strings.Contains(call, "--alias db") {
			t.Fatalf("fixed db alias was added: %q", call)
		}
	}
}

func TestCreateContainersはAliasなしSharedDatabaseを拒否してRollbackする(t *testing.T) {
	runner := &fakeRunner{outputs: []string{
		`{"Config":{"Image":"mysql:8"},"Mounts":[],"NetworkSettings":{"Networks":{"backend":{"Aliases":[]}}}}`,
	}}
	adapter := DockerCLIAdapter{Runner: runner}
	cell := domain.Cell{Name: "123", Containers: domain.Containers{
		NetworkMode: "isolated",
		Network:     "paracell-myapp-123",
		Services: map[string]domain.CellContainer{
			"db": {ContainerName: "paracell-myapp-123-db", SourceContainer: "myapp-db", Database: &domain.DatabaseConfig{Mode: domain.DatabaseModeShared}},
		},
	}}
	template := domain.Template{Containers: domain.ContainerTemplate{Services: map[string]domain.ContainerServiceTemplate{
		"db": {SourceContainer: "myapp-db", Database: &domain.DatabaseConfig{Mode: domain.DatabaseModeShared}},
	}}}

	err := adapter.CreateContainers(context.Background(), cell, template)
	if err == nil || err.Error() != `source database container "myapp-db" for service "db" has no usable network aliases` {
		t.Fatalf("error = %v", err)
	}
	want := []string{
		"docker network create paracell-myapp-123",
		"docker network disconnect -f paracell-myapp-123 paracell-gateway",
		"docker network rm paracell-myapp-123",
	}
	if !reflect.DeepEqual(runner.runCalls, want) {
		t.Fatalf("run calls = %#v, want %#v", runner.runCalls, want)
	}
}

func TestCreateContainersはSharedDatabase接続後の失敗をRollbackする(t *testing.T) {
	inspectErr := errors.New("inspect web failed")
	runner := &fakeRunner{
		outputs: []string{
			`{"Config":{"Image":"mysql:8"},"Mounts":[],"NetworkSettings":{"Networks":{"backend":{"Aliases":["mysql"]}}}}`,
			"",
		},
		outputErrors: []error{nil, inspectErr},
	}
	adapter := DockerCLIAdapter{Runner: runner}
	cell := domain.Cell{Name: "123", Containers: domain.Containers{
		NetworkMode: "isolated", Network: "paracell-myapp-123",
		Services: map[string]domain.CellContainer{
			"db":  {ContainerName: "paracell-myapp-123-db", SourceContainer: "myapp-db", Database: &domain.DatabaseConfig{Mode: domain.DatabaseModeShared}},
			"web": {ContainerName: "paracell-myapp-123-web", SourceContainer: "myapp-web"},
		},
	}}
	template := domain.Template{Containers: domain.ContainerTemplate{Services: map[string]domain.ContainerServiceTemplate{
		"db": {SourceContainer: "myapp-db"}, "web": {SourceContainer: "myapp-web"},
	}}}

	err := adapter.CreateContainers(context.Background(), cell, template)
	if !errors.Is(err, inspectErr) {
		t.Fatalf("error = %v, want %v", err, inspectErr)
	}
	wantTail := []string{
		"docker network disconnect paracell-myapp-123 myapp-db",
		"docker network disconnect -f paracell-myapp-123 paracell-gateway",
		"docker network rm paracell-myapp-123",
	}
	if got := runner.runCalls[len(runner.runCalls)-len(wantTail):]; !reflect.DeepEqual(got, wantTail) {
		t.Fatalf("rollback calls = %#v, want %#v", got, wantTail)
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

func TestCreateContainersはShared工程の途中失敗時に作成済みContainerだけを削除する(t *testing.T) {
	inspectErr := errors.New("inspect web failed")
	runner := &fakeRunner{
		outputs: []string{
			`{"Config":{"Image":"myapp-db:latest"},"Mounts":[],"NetworkSettings":{"Networks":{"myapp_default":{}}}}`,
			"",
		},
		outputErrors: []error{nil, inspectErr},
	}
	adapter := DockerCLIAdapter{Runner: runner, Root: "/project"}
	cell := domain.Cell{Name: "123", Containers: domain.Containers{
		NetworkMode: "shared",
		Services: map[string]domain.CellContainer{
			"db":  {ContainerName: "paracell-myapp-123-db", SourceContainer: "myapp-db"},
			"web": {ContainerName: "paracell-myapp-123-web", SourceContainer: "myapp-web"},
		},
	}}
	template := domain.Template{Containers: domain.ContainerTemplate{Services: map[string]domain.ContainerServiceTemplate{
		"db": {SourceContainer: "myapp-db"}, "web": {SourceContainer: "myapp-web"},
	}}}

	err := adapter.CreateContainers(context.Background(), cell, template)
	if !errors.Is(err, inspectErr) {
		t.Fatalf("error = %v", err)
	}
	wantTail := "docker rm -f paracell-myapp-123-db"
	if len(runner.runCalls) == 0 || runner.runCalls[len(runner.runCalls)-1] != wantTail {
		t.Fatalf("partial container was not removed: %#v", runner.runCalls)
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
	if got := runner.runCalls[2]; got != "docker run -d --name paracell-myapp-123-db --network paracell-myapp-123 --network-alias db --label com.docker.compose.project=paracell-myapp-123 --label com.docker.compose.service=db -e MYSQL_DATABASE=myapp -e MYSQL_USER=app -e MYSQL_PASSWORD=secret -e MYSQL_ROOT_PASSWORD=rootsecret -v paracell-myapp-123-db-var-lib-mysql:/var/lib/mysql:rw mysql:8" {
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
		"docker run -d --name paracell-myapp-123-web --network paracell-myapp-123 --network-alias web --label com.docker.compose.project=paracell-myapp-123 --label com.docker.compose.service=web -e APP_ENV=dev -v paracell-myapp-123-web-app-vendor:/app/vendor myapp-web:latest",
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

func TestCreateContainersはVolumeModeCopyで相対PathのCellSourceをMountする(t *testing.T) {
	runner := &fakeRunner{
		outputs: []string{
			`{"Config":{"Image":"myapp-web:latest"},"Mounts":[{"Type":"bind","Source":"/project","Destination":"/app","RW":true},{"Type":"volume","Name":"myapp_vendor","Source":"/var/lib/docker/volumes/myapp_vendor/_data","Destination":"/app/vendor","RW":true}],"NetworkSettings":{"Networks":{"myapp_default":{}}}}`,
		},
	}
	adapter := DockerCLIAdapter{Runner: runner, Root: "/project"}
	cell := domain.Cell{
		Name:   "123",
		Source: domain.Source{Path: ".paracell/cells/123/source"},
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

	got := runner.runCalls[len(runner.runCalls)-1]
	wantMount := "-v /project/.paracell/cells/123/source:/app"
	if !strings.Contains(got, wantMount) {
		t.Fatalf("run call = %q, want source mount %q", got, wantMount)
	}
}

func TestCreateContainersはComposeが解決したSourcePathからCellSourceをMountする(t *testing.T) {
	runner := &fakeRunner{
		outputs: []string{
			`{"Config":{"Image":"myapp-web:latest","Labels":{"com.docker.compose.project.working_dir":"/Users/user/workspace","com.docker.compose.project.config_files":"/Users/user/workspace/docker-compose.yml","com.docker.compose.service":"web"}},"Mounts":[{"Type":"bind","Source":"/host_mnt/Users/user/workspace/project","Destination":"/app","RW":true},{"Type":"bind","Source":"/var/run/docker.sock","Destination":"/var/run/docker.sock","RW":true},{"Type":"volume","Name":"myapp_vendor","Source":"/var/lib/docker/volumes/myapp_vendor/_data","Destination":"/app/vendor","RW":true}],"NetworkSettings":{"Networks":{"myapp_default":{}}}}`,
			`{"services":{"web":{"volumes":[{"type":"bind","source":"/Users/user/workspace/project","target":"/app"},{"type":"bind","source":"/var/run/docker.sock","target":"/var/run/docker.sock"},{"type":"volume","source":"myapp_vendor","target":"/app/vendor"}]}}}`,
		},
	}
	adapter := DockerCLIAdapter{Runner: runner, Root: "/Users/user/workspace/project"}
	cell := domain.Cell{
		Name:   "123",
		Source: domain.Source{Path: ".paracell/cells/123/source"},
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

	wantOutputCalls := []string{
		`docker inspect -f {{json .}} myapp-web`,
		`docker compose --project-directory /Users/user/workspace -f /Users/user/workspace/docker-compose.yml --profile * config --format json`,
	}
	if !reflect.DeepEqual(runner.outputCalls, wantOutputCalls) {
		t.Fatalf("output calls = %#v, want %#v", runner.outputCalls, wantOutputCalls)
	}
	got := runner.runCalls[len(runner.runCalls)-1]
	wantSourceMount := "-v /Users/user/workspace/project/.paracell/cells/123/source:/app"
	if !strings.Contains(got, wantSourceMount) {
		t.Fatalf("run call = %q, want source mount %q", got, wantSourceMount)
	}
	if strings.Contains(got, "/host_mnt/Users/user/workspace/project:/app") {
		t.Fatalf("run call = %q, Docker daemon側のSource pathを使っている", got)
	}
	wantExternalMount := "-v /var/run/docker.sock:/var/run/docker.sock"
	if !strings.Contains(got, wantExternalMount) {
		t.Fatalf("run call = %q, want external mount %q", got, wantExternalMount)
	}
}

func TestCreateContainersはComposeがProjectRoot配下をBindする場合にCellSource配下へ置き換える(t *testing.T) {
	runner := &fakeRunner{
		outputs: []string{
			`{"Config":{"Image":"myapp-web:latest","Labels":{"com.docker.compose.project.working_dir":"/Users/user/workspace","com.docker.compose.project.config_files":"/Users/user/workspace/docker-compose.yml","com.docker.compose.service":"web"}},"Mounts":[{"Type":"bind","Source":"/host_mnt/Users/user/workspace/project/src","Destination":"/app/src","RW":true},{"Type":"bind","Source":"/Users/user/workspace/shared","Destination":"/shared","RW":false}],"NetworkSettings":{"Networks":{"myapp_default":{}}}}`,
			`{"services":{"web":{"volumes":[{"type":"bind","source":"/Users/user/workspace/project/src","target":"/app/src"},{"type":"bind","source":"/Users/user/workspace/shared","target":"/shared"}]}}}`,
		},
	}
	adapter := DockerCLIAdapter{Runner: runner, Root: "/Users/user/workspace/project"}
	cell := domain.Cell{
		Name:   "123",
		Source: domain.Source{Path: ".paracell/cells/123/source"},
		Containers: domain.Containers{
			NetworkMode: "isolated",
			Services: map[string]domain.CellContainer{
				"web": {
					ContainerName:   "paracell-myapp-123-web",
					SourceContainer: "myapp-web",
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

	got := runner.runCalls[len(runner.runCalls)-1]
	wantSourceMount := "-v /Users/user/workspace/project/.paracell/cells/123/source/src:/app/src"
	if !strings.Contains(got, wantSourceMount) {
		t.Fatalf("run call = %q, want source mount %q", got, wantSourceMount)
	}
	if strings.Contains(got, "/host_mnt/Users/user/workspace/project/src:/app/src") {
		t.Fatalf("run call = %q, Docker daemon側のSource pathを使っている", got)
	}
	wantExternalMount := "-v /Users/user/workspace/shared:/shared:ro"
	if !strings.Contains(got, wantExternalMount) {
		t.Fatalf("run call = %q, want external mount %q", got, wantExternalMount)
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
		"docker network disconnect -f paracell-myapp-123 paracell-gateway",
		"docker network rm paracell-myapp-123",
	}
	if !reflect.DeepEqual(runner.runCalls, want) {
		t.Fatalf("run calls = %#v, want %#v", runner.runCalls, want)
	}
}

func TestCleanContainersはSharedDatabaseを対象CellNetworkからだけ切断する(t *testing.T) {
	runner := &fakeRunner{}
	adapter := DockerCLIAdapter{Runner: runner}
	cell := domain.Cell{Containers: domain.Containers{
		NetworkMode: "isolated",
		Network:     "paracell-myapp-123",
		Services: map[string]domain.CellContainer{
			"db": {
				ContainerName:   "paracell-myapp-123-db",
				SourceContainer: "myapp-db",
				Database:        &domain.DatabaseConfig{Mode: domain.DatabaseModeShared},
			},
			"web": {ContainerName: "paracell-myapp-123-web"},
		},
	}}

	if err := adapter.CleanContainers(context.Background(), cell); err != nil {
		t.Fatalf("CleanContainersでエラーが返った: %v", err)
	}

	want := []string{
		"docker rm -f paracell-myapp-123-web",
		"docker network disconnect paracell-myapp-123 myapp-db",
		"docker network disconnect -f paracell-myapp-123 paracell-gateway",
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

func TestCleanContainersは削除失敗を結合して残りのResourceも削除する(t *testing.T) {
	dbErr := errors.New("db removal failed")
	gatewayErr := errors.New("gateway disconnect failed")
	networkErr := errors.New("network removal failed")
	runner := &fakeRunner{runErrors: map[string]error{
		"docker rm -f paracell-myapp-123-db":                               dbErr,
		"docker network disconnect -f paracell-myapp-123 paracell-gateway": gatewayErr,
		"docker network rm paracell-myapp-123":                             networkErr,
	}}
	adapter := DockerCLIAdapter{Runner: runner}
	cell := domain.Cell{Containers: domain.Containers{
		NetworkMode: "isolated",
		Network:     "paracell-myapp-123",
		Services: map[string]domain.CellContainer{
			"web": {ContainerName: "paracell-myapp-123-web"},
			"db":  {ContainerName: "paracell-myapp-123-db"},
		},
	}}

	err := adapter.CleanContainers(context.Background(), cell)

	for _, want := range []error{dbErr, gatewayErr, networkErr} {
		if !errors.Is(err, want) {
			t.Errorf("error = %v, want errors.Is(_, %v)", err, want)
		}
	}
	wantCalls := []string{
		"docker rm -f paracell-myapp-123-db",
		"docker rm -f paracell-myapp-123-web",
		"docker network disconnect -f paracell-myapp-123 paracell-gateway",
		"docker network rm paracell-myapp-123",
	}
	if !reflect.DeepEqual(runner.runCalls, wantCalls) {
		t.Fatalf("run calls = %#v, want %#v", runner.runCalls, wantCalls)
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
	if name == "docker" && len(args) > 0 && args[len(args)-1] == gatewayContainerName {
		if r.gatewayInspectOutput != nil || r.gatewayInspectError != nil {
			r.outputCalls = append(r.outputCalls, name+" "+joinArgs(args))
			if r.gatewayInspectOutput == nil {
				return "", r.gatewayInspectError
			}
			return *r.gatewayInspectOutput, r.gatewayInspectError
		}
		network := ""
		for i := len(r.runCalls) - 1; i >= 0; i-- {
			const prefix = "docker network create "
			if strings.HasPrefix(r.runCalls[i], prefix) {
				network = strings.TrimPrefix(r.runCalls[i], prefix)
				break
			}
		}
		return `{"Config":{"Labels":{"io.paracell.gateway":"true","io.paracell.gateway.config-version":"3"}},"State":{"Running":true},"NetworkSettings":{"Networks":{"` + network + `":{}}}}`, nil
	}
	r.outputCalls = append(r.outputCalls, name+" "+joinArgs(args))
	out := r.outputs[0]
	r.outputs = r.outputs[1:]
	var err error
	if len(r.outputErrors) > 0 {
		err = r.outputErrors[0]
		r.outputErrors = r.outputErrors[1:]
	}
	return out, err
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
