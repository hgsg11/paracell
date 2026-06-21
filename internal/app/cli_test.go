package app

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hgsg11/paracell/internal/adapter/state"
	viewadapter "github.com/hgsg11/paracell/internal/adapter/view"
	"github.com/hgsg11/paracell/internal/domain"
	"github.com/hgsg11/paracell/internal/usecase"
)

func TestForkコマンドを解析できる(t *testing.T) {
	cmd, err := ParseCommand([]string{"fork", "123", "--template", "webapp"})

	if err != nil {
		t.Fatalf("fork解析でエラーが返った: %v", err)
	}
	if cmd.Kind != CommandFork {
		t.Fatalf("command kind = %q, want %q", cmd.Kind, CommandFork)
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

func TestHelpオプションを解析できる(t *testing.T) {
	cmd, err := ParseCommand([]string{"--help"})

	if err != nil {
		t.Fatalf("help解析でエラーが返った: %v", err)
	}
	if cmd.Kind != CommandHelp {
		t.Fatalf("command kind = %q, want %q", cmd.Kind, CommandHelp)
	}
}

func TestVersionコマンドを解析できる(t *testing.T) {
	cmd, err := ParseCommand([]string{"version"})

	if err != nil {
		t.Fatalf("version解析でエラーが返った: %v", err)
	}
	if cmd.Kind != CommandVersion {
		t.Fatalf("command kind = %q, want %q", cmd.Kind, CommandVersion)
	}
}

func TestVersionオプションを解析できる(t *testing.T) {
	cmd, err := ParseCommand([]string{"--version"})

	if err != nil {
		t.Fatalf("--version解析でエラーが返った: %v", err)
	}
	if cmd.Kind != CommandVersion {
		t.Fatalf("command kind = %q, want %q", cmd.Kind, CommandVersion)
	}
}

func Test引数なしはViewコマンドとして解析する(t *testing.T) {
	cmd, err := ParseCommand([]string{})

	if err != nil {
		t.Fatalf("引数なし解析でエラーが返った: %v", err)
	}
	if cmd.Kind != CommandRoot {
		t.Fatalf("command kind = %q, want %q", cmd.Kind, CommandRoot)
	}
}

func TestLsコマンドは余計な引数があるとエラーにする(t *testing.T) {
	_, err := ParseCommand([]string{"ls", "extra"})

	if err == nil {
		t.Fatal("lsに余計な引数があるのにエラーが返らなかった")
	}
	if err.Error() != "usage: paracell ls" {
		t.Fatalf("error = %q, want %q", err.Error(), "usage: paracell ls")
	}
}

func TestCleanコマンドを解析できる(t *testing.T) {
	cmd, err := ParseCommand([]string{"clean", "123", "--force"})

	if err != nil {
		t.Fatalf("clean解析でエラーが返った: %v", err)
	}
	if cmd.Kind != CommandClean {
		t.Fatalf("command kind = %q, want %q", cmd.Kind, CommandClean)
	}
	if cmd.Cell != "123" {
		t.Fatalf("cell = %q, want %q", cmd.Cell, "123")
	}
	if !cmd.Force {
		t.Fatal("force = false, want true")
	}
}

func TestPendingコマンドを解析できる(t *testing.T) {
	cmd, err := ParseCommand([]string{"pending"})

	if err != nil {
		t.Fatalf("pending解析でエラーが返った: %v", err)
	}
	if cmd.Kind != CommandPending {
		t.Fatalf("command kind = %q, want %q", cmd.Kind, CommandPending)
	}
}

func TestReadyコマンドを解析できる(t *testing.T) {
	cmd, err := ParseCommand([]string{"ready"})

	if err != nil {
		t.Fatalf("ready解析でエラーが返った: %v", err)
	}
	if cmd.Kind != CommandReady {
		t.Fatalf("command kind = %q, want %q", cmd.Kind, CommandReady)
	}
}

func TestRunはHelpでUsageを出力する(t *testing.T) {
	dir := t.TempDir()

	output, err := captureStdout(func() error {
		return Run(context.Background(), []string{"--help"}, dir)
	})

	if err != nil {
		t.Fatalf("Runでエラーが返った: %v", err)
	}
	want := "usage: paracell [init|fork|ls|view|clean|version|help]\n"
	want = "usage: paracell [init|fork|ls|view|clean|pending|ready|version|help]\n"
	if output != want {
		t.Fatalf("output = %q, want %q", output, want)
	}
}

func TestRunはVersionを出力する(t *testing.T) {
	dir := t.TempDir()
	originalVersion := Version
	originalCommit := Commit
	originalDate := BuildDate
	defer func() {
		Version = originalVersion
		Commit = originalCommit
		BuildDate = originalDate
	}()
	Version = "v0.1.6"
	Commit = "abc1234"
	BuildDate = "2026-06-08T12:34:56Z"

	output, err := captureStdout(func() error {
		return Run(context.Background(), []string{"version"}, dir)
	})

	if err != nil {
		t.Fatalf("Runでエラーが返った: %v", err)
	}
	want := "paracell v0.1.6\ncommit: abc1234\nbuilt: 2026-06-08T12:34:56Z\n"
	if output != want {
		t.Fatalf("output = %q, want %q", output, want)
	}
}

func TestRunはLsでStateのCell一覧を出力する(t *testing.T) {
	dir := t.TempDir()
	store := state.JSONCellStateAdapter{Path: filepath.Join(dir, ".paracell", "state.json")}
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
	if _, err := os.Stat(filepath.Join(dir, ".paracell", "state.json")); !os.IsNotExist(err) {
		t.Fatalf("state.json existence error = %v, want not exist", err)
	}
}

func TestRunはCellSource内からLsしてもProjectRootのStateを読む(t *testing.T) {
	dir := t.TempDir()
	store := state.JSONCellStateAdapter{Path: filepath.Join(dir, ".paracell", "state.json")}
	if err := store.SaveCells(context.Background(), []domain.Cell{
		{Name: "123", Template: "default"},
		{Name: "456", Template: "webapp"},
	}); err != nil {
		t.Fatalf("state保存でエラーが返った: %v", err)
	}
	cellSource := filepath.Join(dir, ".paracell", "cells", "123", "source")
	if err := os.MkdirAll(cellSource, 0o755); err != nil {
		t.Fatalf("cell sourceを作れなかった: %v", err)
	}

	output, err := captureStdout(func() error {
		return Run(context.Background(), []string{"ls"}, cellSource)
	})

	if err != nil {
		t.Fatalf("Runでエラーが返った: %v", err)
	}
	want := "NAME\tTEMPLATE\n123\tdefault\n456\twebapp\n"
	if output != want {
		t.Fatalf("output = %q, want %q", output, want)
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
	store := state.JSONCellStateAdapter{Path: filepath.Join(dir, ".paracell", "state.json")}
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
	originalClean := runClean
	defer func() { runClean = originalClean }()

	var got []domain.Cell
	runView = func(ctx context.Context, cells []domain.Cell, reload func() ([]domain.Cell, error), enter func(domain.Cell) tea.Cmd, exit func() error, clean func(domain.Cell) error, markDone func(domain.Cell) (domain.Cell, error)) (viewadapter.Result, error) {
		_ = ctx
		_ = reload
		_ = enter
		_ = exit
		_ = clean
		_ = markDone
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
	runClean = func(ctx context.Context, cfg usecase.ConfigPort, source usecase.SourceProviderFactory, container usecase.ContainerProviderFactory, session usecase.SessionProviderFactory, state usecase.CellStatePort, cell domain.Cell) error {
		_ = ctx
		_ = cfg
		_ = source
		_ = container
		_ = session
		_ = state
		_ = cell
		t.Fatal("cleanが呼ばれた")
		return nil
	}

	if err := Run(context.Background(), []string{"view"}, dir); err != nil {
		t.Fatalf("Runでエラーが返った: %v", err)
	}
	want := []domain.Cell{
		func() domain.Cell {
			cell := domain.Cell{ID: "cell-1", Name: "123", Template: "default"}
			if err := cell.SetStatus(domain.Ready); err != nil {
				t.Fatalf("cell status設定でエラーが返った: %v", err)
			}
			return cell
		}(),
		func() domain.Cell {
			cell := domain.Cell{ID: "cell-2", Name: "456", Template: "webapp"}
			if err := cell.SetStatus(domain.Ready); err != nil {
				t.Fatalf("cell status設定でエラーが返った: %v", err)
			}
			return cell
		}(),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("cells = %#v, want %#v", got, want)
	}
}

func TestRunは引数なしでRootSessionEnterを実行する(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "paracell.yaml")
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

	originalRoot := runEnterRoot
	defer func() { runEnterRoot = originalRoot }()
	originalView := runView
	defer func() { runView = originalView }()

	called := false
	var gotProject string
	runEnterRoot = func(ctx context.Context, cfg usecase.ConfigPort, factory usecase.SessionProviderFactory) error {
		loaded, err := cfg.Load(ctx, nil)
		if err != nil {
			return err
		}
		_ = factory
		called = true
		gotProject = loaded.Project.Name
		return nil
	}
	runView = func(ctx context.Context, cells []domain.Cell, reload func() ([]domain.Cell, error), enter func(domain.Cell) tea.Cmd, exit func() error, clean func(domain.Cell) error, markDone func(domain.Cell) (domain.Cell, error)) (viewadapter.Result, error) {
		t.Fatal("viewが呼ばれた")
		return viewadapter.Result{}, nil
	}

	if err := Run(context.Background(), nil, dir); err != nil {
		t.Fatalf("Runでエラーが返った: %v", err)
	}
	if !called {
		t.Fatal("root session enterが呼ばれなかった")
	}
	if gotProject != "myapp" {
		t.Fatalf("project = %q, want %q", gotProject, "myapp")
	}
}

func TestRunはViewコマンドで引き続きTUIを起動する(t *testing.T) {
	dir := t.TempDir()
	store := state.JSONCellStateAdapter{Path: filepath.Join(dir, ".paracell", "state.json")}
	if err := store.SaveCells(context.Background(), []domain.Cell{}); err != nil {
		t.Fatalf("state保存でエラーが返った: %v", err)
	}

	originalRoot := runEnterRoot
	defer func() { runEnterRoot = originalRoot }()
	originalView := runView
	defer func() { runView = originalView }()

	runEnterRoot = func(ctx context.Context, cfg usecase.ConfigPort, factory usecase.SessionProviderFactory) error {
		t.Fatal("root enterが呼ばれた")
		return nil
	}
	called := false
	runView = func(ctx context.Context, cells []domain.Cell, reload func() ([]domain.Cell, error), enter func(domain.Cell) tea.Cmd, exit func() error, clean func(domain.Cell) error, markDone func(domain.Cell) (domain.Cell, error)) (viewadapter.Result, error) {
		_ = ctx
		_ = cells
		_ = reload
		_ = enter
		_ = exit
		_ = clean
		_ = markDone
		called = true
		return viewadapter.Result{Action: viewadapter.ActionQuit}, nil
	}

	if err := Run(context.Background(), []string{"view"}, dir); err != nil {
		t.Fatalf("Runでエラーが返った: %v", err)
	}
	if !called {
		t.Fatal("viewが呼ばれなかった")
	}
}

func TestRunはViewでEnterしたCellをEnter処理に渡す(t *testing.T) {
	dir := t.TempDir()
	store := state.JSONCellStateAdapter{Path: filepath.Join(dir, ".paracell", "state.json")}
	if err := store.SaveCells(context.Background(), []domain.Cell{
		{ID: "cell-1", Name: "123", Template: "default"},
	}); err != nil {
		t.Fatalf("state保存でエラーが返った: %v", err)
	}
	configPath := filepath.Join(dir, "paracell.yaml")
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
	originalClean := runClean
	defer func() { runClean = originalClean }()

	var entered domain.Cell
	runView = func(ctx context.Context, cells []domain.Cell, reload func() ([]domain.Cell, error), enter func(domain.Cell) tea.Cmd, exit func() error, clean func(domain.Cell) error, markDone func(domain.Cell) (domain.Cell, error)) (viewadapter.Result, error) {
		_ = ctx
		_ = reload
		_ = exit
		_ = clean
		_ = markDone
		cmd := enter(cells[0])
		if cmd == nil {
			t.Fatal("enterでコマンドが返らなかった")
		}
		_ = cmd
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
	runClean = func(ctx context.Context, cfg usecase.ConfigPort, source usecase.SourceProviderFactory, container usecase.ContainerProviderFactory, session usecase.SessionProviderFactory, state usecase.CellStatePort, cell domain.Cell) error {
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

func TestRunはViewでddしたCellをClean処理に渡す(t *testing.T) {
	dir := t.TempDir()
	store := state.JSONCellStateAdapter{Path: filepath.Join(dir, ".paracell", "state.json")}
	if err := store.SaveCells(context.Background(), []domain.Cell{
		{ID: "cell-1", Name: "123", Template: "default"},
	}); err != nil {
		t.Fatalf("state保存でエラーが返った: %v", err)
	}
	configPath := filepath.Join(dir, "paracell.yaml")
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
	originalClean := runClean
	defer func() { runClean = originalClean }()

	var deleted domain.Cell
	runView = func(ctx context.Context, cells []domain.Cell, reload func() ([]domain.Cell, error), enter func(domain.Cell) tea.Cmd, exit func() error, clean func(domain.Cell) error, markDone func(domain.Cell) (domain.Cell, error)) (viewadapter.Result, error) {
		_ = ctx
		_ = reload
		_ = enter
		_ = exit
		_ = markDone
		if err := clean(cells[0]); err != nil {
			t.Fatalf("cleanでエラーが返った: %v", err)
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
	runClean = func(ctx context.Context, cfg usecase.ConfigPort, source usecase.SourceProviderFactory, container usecase.ContainerProviderFactory, session usecase.SessionProviderFactory, state usecase.CellStatePort, cell domain.Cell) error {
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

func TestRunExitはTMUX内ならDetachClientを実行する(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-1000/default,123,0")
	runner := &fakeRunner{outputs: map[string]string{}}

	if err := runExit(context.Background(), runner); err != nil {
		t.Fatalf("runExitでエラーが返った: %v", err)
	}
	want := []string{"tmux detach-client"}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestRunExitはTMUX外なら何もしない(t *testing.T) {
	t.Setenv("TMUX", "")
	runner := &fakeRunner{outputs: map[string]string{}}

	if err := runExit(context.Background(), runner); err != nil {
		t.Fatalf("runExitでエラーが返った: %v", err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("calls = %#v, want empty", runner.calls)
	}
}

func TestRunはCreateでProvidersがない設定をエラーにする(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "paracell.yaml")
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

	err := Run(context.Background(), []string{"fork", "123", "--template", "default"}, dir)

	if err == nil {
		t.Fatal("providersがないのにエラーが返らなかった")
	}
	if err.Error() != "providers.source is required" {
		t.Fatalf("error = %q, want %q", err.Error(), "providers.source is required")
	}
}

func TestRunはCreateでContainerProviderがなくてもDockerを実行しない(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "paracell.yaml")
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

	err := Run(context.Background(), []string{"fork", "123", "--template", "default"}, dir)

	if err == nil {
		t.Fatal("git/tmuxが実行できない環境なのにエラーが返らなかった")
	}
	if err.Error() == `exec: "docker": executable file not found in $PATH` {
		t.Fatalf("container provider未設定なのにdockerが実行された: %v", err)
	}
}

func TestRunはCleanでContainerProviderがなくてもDockerを実行しない(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "paracell.yaml")
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
	store := state.JSONCellStateAdapter{Path: filepath.Join(dir, ".paracell", "state.json")}
	if err := store.SaveCells(context.Background(), []domain.Cell{
		{
			ID:    "cell-1",
			Issue: "123",
			Name:  "123",
			Source: domain.Source{
				Path: filepath.Join(dir, "missing-source"),
			},
			Containers: domain.Containers{
				Network: "paracell-myapp-123",
				Services: map[string]domain.CellContainer{
					"web": {ContainerName: "paracell-myapp-123-web"},
				},
			},
			Session: domain.Session{Name: "paracell-myapp-123"},
		},
	}); err != nil {
		t.Fatalf("state保存でエラーが返った: %v", err)
	}

	err := Run(context.Background(), []string{"clean", "123"}, dir)

	if err == nil {
		t.Fatal("tmux/gitが実行できない環境なのにエラーが返らなかった")
	}
	if err.Error() == `exec: "docker": executable file not found in $PATH` {
		t.Fatalf("container provider未設定なのにdockerが実行された: %v", err)
	}
}

func TestRunはReadyでPARACELL_CELLのStatusを更新する(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PARACELL_CELL", "123")
	store := state.JSONCellStateAdapter{Path: filepath.Join(dir, ".paracell", "state.json")}
	if err := store.SaveCells(context.Background(), []domain.Cell{
		{ID: "cell-1", Issue: "123", Name: "123"},
	}); err != nil {
		t.Fatalf("state保存でエラーが返った: %v", err)
	}

	if err := Run(context.Background(), []string{"ready"}, dir); err != nil {
		t.Fatalf("Runでエラーが返った: %v", err)
	}

	cells, err := store.LoadCells(context.Background())
	if err != nil {
		t.Fatalf("state読み込みでエラーが返った: %v", err)
	}
	if got := cells[0].Status(); got != domain.Ready {
		t.Fatalf("Status = %q, want %q", got, domain.Ready)
	}
}

func TestRunはPendingでPARACELL_CELLがないと失敗する(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PARACELL_CELL", "")

	err := Run(context.Background(), []string{"pending"}, dir)

	if err == nil {
		t.Fatal("PARACELL_CELLがないのにエラーが返らなかった")
	}
	if err.Error() != "PARACELL_CELL is required" {
		t.Fatalf("error = %q, want %q", err.Error(), "PARACELL_CELL is required")
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

type fakeRunner struct {
	calls   []string
	outputs map[string]string
}

func (r *fakeRunner) Run(ctx context.Context, name string, args ...string) error {
	_ = ctx
	r.calls = append(r.calls, name+" "+joinArgs(args))
	return nil
}

func (r *fakeRunner) Output(ctx context.Context, name string, args ...string) (string, error) {
	_ = ctx
	return r.outputs[name+" "+joinArgs(args)], nil
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
