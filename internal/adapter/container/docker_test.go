package container

import (
	"reflect"
	"testing"
)

func TestDockerRun引数を組み立てられる(t *testing.T) {
	spec := RunSpec{
		Name:       "pdev-myapp-123-web",
		Image:      "nginx:latest",
		Network:    "pdev-myapp-123",
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
		"--name", "pdev-myapp-123-web",
		"--network", "pdev-myapp-123",
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
