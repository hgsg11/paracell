package app

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/shige1114/paradev/internal/adapter/state"
	viewadapter "github.com/shige1114/paradev/internal/adapter/view"
	"github.com/shige1114/paradev/internal/domain"
	"github.com/shige1114/paradev/internal/usecase"
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

func TestViewコマンドを解析できる(t *testing.T) {
	cmd, err := ParseCommand([]string{"view"})

	if err != nil {
		t.Fatalf("view解析でエラーが返った: %v", err)
	}
	if cmd.Kind != CommandView {
		t.Fatalf("command kind = %q, want %q", cmd.Kind, CommandView)
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

func TestRunはViewでCell一覧をTUIに渡す(t *testing.T) {
	dir := t.TempDir()
	store := state.JSONCellStateAdapter{Path: filepath.Join(dir, ".pdev", "state.json")}
	if err := store.SaveCells(context.Background(), []domain.Cell{
		{ID: "cell-1", Name: "123", Template: "default"},
		{ID: "cell-2", Name: "456", Template: "webapp"},
	}); err != nil {
		t.Fatalf("state保存でエラーが返った: %v", err)
	}

	original := runView
	defer func() { runView = original }()
	originalEnter := runEnter
	defer func() { runEnter = originalEnter }()
	originalDelete := runDelete
	defer func() { runDelete = originalDelete }()

	var got []domain.Cell
	runView = func(ctx context.Context, cells []domain.Cell, enter func(domain.Cell) error, delete func(domain.Cell) error) (viewadapter.Result, error) {
		_ = ctx
		_ = enter
		_ = delete
		got = append([]domain.Cell(nil), cells...)
		return viewadapter.Result{Action: viewadapter.ActionQuit}, nil
	}
	runEnter = func(ctx context.Context, cfg usecase.ConfigPort, factory usecase.SessionProviderFactory, cell domain.Cell) error {
		_ = ctx
		_ = cfg
		_ = factory
		_ = cell
		t.Fatal("enterが呼ばれた")
		return nil
	}
	runDelete = func(ctx context.Context, cfg usecase.ConfigPort, source usecase.SourceProviderFactory, container usecase.ContainerProviderFactory, session usecase.SessionProviderFactory, state usecase.CellStatePort, cell domain.Cell) error {
		_ = ctx
		_ = cfg
		_ = source
		_ = container
		_ = session
		_ = state
		_ = cell
		t.Fatal("deleteが呼ばれた")
		return nil
	}

	if err := Run(context.Background(), []string{"view"}, dir); err != nil {
		t.Fatalf("Runでエラーが返った: %v", err)
	}
	want := []domain.Cell{
		{ID: "cell-1", Name: "123", Template: "default"},
		{ID: "cell-2", Name: "456", Template: "webapp"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("cells = %#v, want %#v", got, want)
	}
}

func TestRunはViewでEnterしたCellをEnter処理に渡す(t *testing.T) {
	dir := t.TempDir()
	store := state.JSONCellStateAdapter{Path: filepath.Join(dir, ".pdev", "state.json")}
	if err := store.SaveCells(context.Background(), []domain.Cell{
		{ID: "cell-1", Name: "123", Template: "default"},
	}); err != nil {
		t.Fatalf("state保存でエラーが返った: %v", err)
	}
	configPath := filepath.Join(dir, ".pdev.yml")
	content := []byte(`project:
  name: myapp
providers:
  source: git
  session: tmux
templates: {}
`)
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		t.Fatalf("設定を書けなかった: %v", err)
	}

	originalView := runView
	defer func() { runView = originalView }()
	originalEnter := runEnter
	defer func() { runEnter = originalEnter }()
	originalDelete := runDelete
	defer func() { runDelete = originalDelete }()

	var entered domain.Cell
	runView = func(ctx context.Context, cells []domain.Cell, enter func(domain.Cell) error, delete func(domain.Cell) error) (viewadapter.Result, error) {
		_ = ctx
		_ = delete
		if err := enter(cells[0]); err != nil {
			t.Fatalf("enterでエラーが返った: %v", err)
		}
		entered = cells[0]
		return viewadapter.Result{
			Action: viewadapter.ActionEnter,
			Cell:   cells[0],
		}, nil
	}
	runEnter = func(ctx context.Context, cfg usecase.ConfigPort, factory usecase.SessionProviderFactory, cell domain.Cell) error {
		_ = ctx
		_ = cfg
		_ = factory
		entered = cell
		return nil
	}
	runDelete = func(ctx context.Context, cfg usecase.ConfigPort, source usecase.SourceProviderFactory, container usecase.ContainerProviderFactory, session usecase.SessionProviderFactory, state usecase.CellStatePort, cell domain.Cell) error {
		_ = ctx
		_ = cfg
		_ = source
		_ = container
		_ = session
		_ = state
		entered = cell
		return nil
	}

	if err := Run(context.Background(), []string{"view"}, dir); err != nil {
		t.Fatalf("Runでエラーが返った: %v", err)
	}
	if entered.Name != "123" {
		t.Fatalf("entered cell = %#v, want name %q", entered, "123")
	}
}

func TestRunはViewでddしたCellをDelete処理に渡す(t *testing.T) {
	dir := t.TempDir()
	store := state.JSONCellStateAdapter{Path: filepath.Join(dir, ".pdev", "state.json")}
	if err := store.SaveCells(context.Background(), []domain.Cell{
		{ID: "cell-1", Name: "123", Template: "default"},
	}); err != nil {
		t.Fatalf("state保存でエラーが返った: %v", err)
	}
	configPath := filepath.Join(dir, ".pdev.yml")
	content := []byte(`project:
  name: myapp
providers:
  source: git
  session: tmux
templates: {}
`)
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		t.Fatalf("設定を書けなかった: %v", err)
	}

	originalView := runView
	defer func() { runView = originalView }()
	originalEnter := runEnter
	defer func() { runEnter = originalEnter }()
	originalDelete := runDelete
	defer func() { runDelete = originalDelete }()

	var deleted domain.Cell
	runView = func(ctx context.Context, cells []domain.Cell, enter func(domain.Cell) error, delete func(domain.Cell) error) (viewadapter.Result, error) {
		_ = ctx
		_ = enter
		if err := delete(cells[0]); err != nil {
			t.Fatalf("deleteでエラーが返った: %v", err)
		}
		deleted = cells[0]
		return viewadapter.Result{Action: viewadapter.ActionQuit}, nil
	}
	runEnter = func(ctx context.Context, cfg usecase.ConfigPort, factory usecase.SessionProviderFactory, cell domain.Cell) error {
		_ = ctx
		_ = cfg
		_ = factory
		_ = cell
		return nil
	}
	runDelete = func(ctx context.Context, cfg usecase.ConfigPort, source usecase.SourceProviderFactory, container usecase.ContainerProviderFactory, session usecase.SessionProviderFactory, state usecase.CellStatePort, cell domain.Cell) error {
		_ = ctx
		_ = cfg
		_ = source
		_ = container
		_ = session
		_ = state
		deleted = cell
		return nil
	}

	if err := Run(context.Background(), []string{"view"}, dir); err != nil {
		t.Fatalf("Runでエラーが返った: %v", err)
	}
	if deleted.Name != "123" {
		t.Fatalf("deleted cell = %#v, want name %q", deleted, "123")
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

func TestRunはCreateでContainerProviderがなくてもDockerを実行しない(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".pdev.yml")
	content := []byte(`project:
  name: myapp
providers:
  source: git
  session: tmux
templates:
  default:
    repository:
      branchPrefix: feat/
      base: main
    containers:
      services:
        web:
          sourceContainer: myapp-web
    session:
      windows: []
`)
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		t.Fatalf("設定を書けなかった: %v", err)
	}

	err := Run(context.Background(), []string{"create", "123", "--template", "default"}, dir)

	if err == nil {
		t.Fatal("git/tmuxが実行できない環境なのにエラーが返らなかった")
	}
	if err.Error() == `exec: "docker": executable file not found in $PATH` {
		t.Fatalf("container provider未設定なのにdockerが実行された: %v", err)
	}
}

func TestRunはRemoveでContainerProviderがなくてもDockerを実行しない(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".pdev.yml")
	content := []byte(`project:
  name: myapp
providers:
  source: git
  session: tmux
templates: {}
`)
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		t.Fatalf("設定を書けなかった: %v", err)
	}
	store := state.JSONCellStateAdapter{Path: filepath.Join(dir, ".pdev", "state.json")}
	if err := store.SaveCells(context.Background(), []domain.Cell{
		{
			ID:    "cell-1",
			Issue: "123",
			Name:  "123",
			Source: domain.Source{
				Path: filepath.Join(dir, "missing-source"),
			},
			Containers: domain.Containers{
				Network: "pdev-myapp-123",
				Services: map[string]domain.CellContainer{
					"web": {ContainerName: "pdev-myapp-123-web"},
				},
			},
			Session: domain.Session{Name: "pdev-myapp-123"},
		},
	}); err != nil {
		t.Fatalf("state保存でエラーが返った: %v", err)
	}

	err := Run(context.Background(), []string{"remove", "123"}, dir)

	if err == nil {
		t.Fatal("tmux/gitが実行できない環境なのにエラーが返らなかった")
	}
	if err.Error() == `exec: "docker": executable file not found in $PATH` {
		t.Fatalf("container provider未設定なのにdockerが実行された: %v", err)
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
