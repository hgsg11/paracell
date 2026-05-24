package app

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/shige1114/paradev/internal/adapter/state"
	"github.com/shige1114/paradev/internal/domain"
)

func TestCreateコマンドを解析できる(t *testing.T) {
	cmd, err := ParseCommand([]string{"create", "123", "--template", "webapp"})

	if err != nil {
		t.Fatalf("create解析でエラーが返った: %v", err)
	}
	if cmd.Kind != CommandCreate {
		t.Fatalf("command kind = %q, want %q", cmd.Kind, CommandCreate)
	}
	if cmd.Issue != "123" {
		t.Fatalf("issue = %q, want %q", cmd.Issue, "123")
	}
	if cmd.Template != "webapp" {
		t.Fatalf("template = %q, want %q", cmd.Template, "webapp")
	}
}

func TestInitコマンドを解析できる(t *testing.T) {
	cmd, err := ParseCommand([]string{"init"})

	if err != nil {
		t.Fatalf("init解析でエラーが返った: %v", err)
	}
	if cmd.Kind != CommandInit {
		t.Fatalf("command kind = %q, want %q", cmd.Kind, CommandInit)
	}
}

func TestLsコマンドを解析できる(t *testing.T) {
	cmd, err := ParseCommand([]string{"ls"})

	if err != nil {
		t.Fatalf("ls解析でエラーが返った: %v", err)
	}
	if cmd.Kind != CommandList {
		t.Fatalf("command kind = %q, want %q", cmd.Kind, CommandList)
	}
}

func TestLsコマンドは余計な引数があるとエラーにする(t *testing.T) {
	_, err := ParseCommand([]string{"ls", "extra"})

	if err == nil {
		t.Fatal("lsに余計な引数があるのにエラーが返らなかった")
	}
	if err.Error() != "usage: pdev ls" {
		t.Fatalf("error = %q, want %q", err.Error(), "usage: pdev ls")
	}
}

func TestRemoveコマンドを解析できる(t *testing.T) {
	cmd, err := ParseCommand([]string{"remove", "123", "--force"})

	if err != nil {
		t.Fatalf("remove解析でエラーが返った: %v", err)
	}
	if cmd.Kind != CommandRemove {
		t.Fatalf("command kind = %q, want %q", cmd.Kind, CommandRemove)
	}
	if cmd.Cell != "123" {
		t.Fatalf("cell = %q, want %q", cmd.Cell, "123")
	}
	if !cmd.Force {
		t.Fatal("force = false, want true")
	}
}

func TestRunはLsでStateのCell一覧を出力する(t *testing.T) {
	dir := t.TempDir()
	store := state.JSONCellStateAdapter{Path: filepath.Join(dir, ".pdev", "state.json")}
	if err := store.SaveCells(context.Background(), []domain.Cell{
		{Name: "123", Template: "default"},
		{Name: "456", Template: "webapp"},
	}); err != nil {
		t.Fatalf("state保存でエラーが返った: %v", err)
	}

	output, err := captureStdout(func() error {
		return Run(context.Background(), []string{"ls"}, dir)
	})

	if err != nil {
		t.Fatalf("Runでエラーが返った: %v", err)
	}
	want := "NAME\tTEMPLATE\n123\tdefault\n456\twebapp\n"
	if output != want {
		t.Fatalf("output = %q, want %q", output, want)
	}
}

func TestRunはLsでStateがなくてもヘッダーだけ出力する(t *testing.T) {
	dir := t.TempDir()

	output, err := captureStdout(func() error {
		return Run(context.Background(), []string{"ls"}, dir)
	})

	if err != nil {
		t.Fatalf("Runでエラーが返った: %v", err)
	}
	want := "NAME\tTEMPLATE\n"
	if output != want {
		t.Fatalf("output = %q, want %q", output, want)
	}
	if _, err := os.Stat(filepath.Join(dir, ".pdev", "state.json")); !os.IsNotExist(err) {
		t.Fatalf("state.json existence error = %v, want not exist", err)
	}
}

func TestRunはLsでPdevYmlがなくても成功する(t *testing.T) {
	dir := t.TempDir()

	output, err := captureStdout(func() error {
		return Run(context.Background(), []string{"ls"}, dir)
	})

	if err != nil {
		t.Fatalf("Runでエラーが返った: %v", err)
	}
	if output != "NAME\tTEMPLATE\n" {
		t.Fatalf("output = %q, want %q", output, "NAME\tTEMPLATE\n")
	}
}

func TestRunはCreateでProvidersがない設定をエラーにする(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".pdev.yml")
	content := []byte(`project:
  name: myapp
templates:
  default:
    repository:
      branchPrefix: feat/
      base: main
    containers:
      services: {}
    session:
      windows: []
`)
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		t.Fatalf("設定を書けなかった: %v", err)
	}

	err := Run(context.Background(), []string{"create", "123", "--template", "default"}, dir)

	if err == nil {
		t.Fatal("providersがないのにエラーが返らなかった")
	}
	if err.Error() != "providers.source is required" {
		t.Fatalf("error = %q, want %q", err.Error(), "providers.source is required")
	}
}

func captureStdout(fn func() error) (string, error) {
	original := os.Stdout
	read, write, err := os.Pipe()
	if err != nil {
		return "", err
	}
	os.Stdout = write
	defer func() {
		os.Stdout = original
	}()

	runErr := fn()
	closeErr := write.Close()
	output, readErr := io.ReadAll(read)
	if closeErr != nil && runErr == nil {
		runErr = closeErr
	}
	if readErr != nil && runErr == nil {
		runErr = readErr
	}
	return string(output), runErr
}

func Test未対応コマンドはエラーにする(t *testing.T) {
	_, err := ParseCommand([]string{"list"})

	if err == nil {
		t.Fatal("未対応コマンドなのにエラーが返らなかった")
	}
}
