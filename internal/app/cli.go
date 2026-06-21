package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	celladapter "github.com/hgsg11/paracell/internal/adapter/cell"
	"github.com/hgsg11/paracell/internal/adapter/config"
	"github.com/hgsg11/paracell/internal/adapter/files"
	"github.com/hgsg11/paracell/internal/adapter/id"
	"github.com/hgsg11/paracell/internal/adapter/output"
	"github.com/hgsg11/paracell/internal/adapter/provider"
	"github.com/hgsg11/paracell/internal/adapter/state"
	"github.com/hgsg11/paracell/internal/adapter/system"
	viewadapter "github.com/hgsg11/paracell/internal/adapter/view"
	"github.com/hgsg11/paracell/internal/domain"
	"github.com/hgsg11/paracell/internal/usecase"
)

var (
	runView  = viewadapter.Run
	runEnter = func(ctx context.Context, cfg usecase.ConfigPort, factory usecase.SessionProviderFactory, cell domain.Cell) error {
		uc := usecase.EnterCellUseCase{
			Config:         cfg,
			SessionFactory: factory,
		}
		_, err := uc.Execute(ctx, usecase.EnterCellInput{Cell: cell})
		return err
	}
	runMarkDone = func(ctx context.Context, state usecase.CellStatePort, cell domain.Cell) (domain.Cell, error) {
		uc := usecase.MarkCellDoneUseCase{State: state}
		return uc.Execute(ctx, usecase.MarkCellDoneInput{Cell: cell.Name})
	}
	runSetStatus = func(ctx context.Context, state usecase.CellStatePort, cellName string, status domain.CellStatus) (domain.Cell, error) {
		uc := usecase.SetCellStatusUseCase{State: state}
		return uc.Execute(ctx, usecase.SetCellStatusInput{Cell: cellName, Status: status})
	}
	runExit = func(ctx context.Context, runner system.Runner) error {
		if os.Getenv("TMUX") == "" {
			return nil
		}
		return runner.Run(ctx, "tmux", "detach-client")
	}
	runEnterCmd = func(ctx context.Context, cfg usecase.ConfigPort, cell domain.Cell) (*exec.Cmd, error) {
		loaded, err := cfg.Load(ctx, nil)
		if err != nil {
			return nil, err
		}
		if loaded.Providers.Session != "tmux" {
			return nil, fmt.Errorf("unsupported providers.session %q", loaded.Providers.Session)
		}
		if os.Getenv("TMUX") != "" {
			return exec.CommandContext(ctx, "tmux", "switch-client", "-t", cell.Session.Name), nil
		}
		return exec.CommandContext(ctx, "tmux", "attach-session", "-t", cell.Session.Name), nil
	}
)

var runClean = func(ctx context.Context, cfg usecase.ConfigPort, source usecase.SourceProviderFactory, container usecase.ContainerProviderFactory, session usecase.SessionProviderFactory, state usecase.CellStatePort, cell domain.Cell) error {
	uc := usecase.CleanCellUseCase{
		Config:           cfg,
		State:            state,
		SourceFactory:    source,
		ContainerFactory: container,
		SessionFactory:   session,
	}
	return uc.Execute(ctx, usecase.CleanCellInput{Cell: cell.Name})
}

type CommandKind string

const AppName = "paracell"

const (
	CommandInit    CommandKind = "init"
	CommandFork    CommandKind = "fork"
	CommandClean   CommandKind = "clean"
	CommandList    CommandKind = "ls"
	CommandPending CommandKind = "pending"
	CommandReady   CommandKind = "ready"
	CommandView    CommandKind = "view"
	CommandVersion CommandKind = "version"
	CommandHelp    CommandKind = "help"
)

const usage = "usage: paracell [init|fork|ls|view|clean|pending|ready|version|help]\n"

var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

type Command struct {
	Kind     CommandKind
	Issue    string
	Template string
	Cell     string
	Force    bool
}

func ParseCommand(args []string) (Command, error) {
	if len(args) == 0 {
		return Command{Kind: CommandView}, nil
	}
	switch args[0] {
	case "--version":
		if len(args) != 1 {
			return Command{}, errors.New("usage: paracell --version")
		}
		return Command{Kind: CommandVersion}, nil
	case "--help", "-h", "help":
		if len(args) != 1 {
			return Command{}, errors.New("usage: paracell help")
		}
		return Command{Kind: CommandHelp}, nil
	case "init":
		if len(args) != 1 {
			return Command{}, errors.New("usage: paracell init")
		}
		return Command{Kind: CommandInit}, nil
	case "ls":
		if len(args) != 1 {
			return Command{}, errors.New("usage: paracell ls")
		}
		return Command{Kind: CommandList}, nil
	case "view":
		if len(args) != 1 {
			return Command{}, errors.New("usage: paracell view")
		}
		return Command{Kind: CommandView}, nil
	case "pending":
		if len(args) != 1 {
			return Command{}, errors.New("usage: paracell pending")
		}
		return Command{Kind: CommandPending}, nil
	case "ready":
		if len(args) != 1 {
			return Command{}, errors.New("usage: paracell ready")
		}
		return Command{Kind: CommandReady}, nil
	case "version":
		if len(args) != 1 {
			return Command{}, errors.New("usage: paracell version")
		}
		return Command{Kind: CommandVersion}, nil
	case "fork":
		if len(args) != 4 || args[2] != "--template" || args[1] == "" || args[3] == "" {
			return Command{}, errors.New("usage: paracell fork <issue> --template <template>")
		}
		return Command{Kind: CommandFork, Issue: args[1], Template: args[3]}, nil
	case "clean":
		if len(args) != 2 && !(len(args) == 3 && args[2] == "--force") {
			return Command{}, errors.New("usage: paracell clean <cell> [--force]")
		}
		return Command{Kind: CommandClean, Cell: args[1], Force: len(args) == 3}, nil
	default:
		return Command{}, fmt.Errorf("unsupported command %q", args[0])
	}
}

func Run(ctx context.Context, args []string, workdir string) error {
	cmd, err := ParseCommand(args)
	if err != nil {
		return err
	}
	workdir = projectRootForWorkdir(workdir)
	runner := system.OSCommandRunner{Dir: workdir}
	quietRunner := system.CaptureRunner{Dir: workdir}
	configAdapter := config.YAMLConfigAdapter{Path: filepath.Join(workdir, "paracell.yaml")}
	stateAdapter := state.JSONCellStateAdapter{Path: filepath.Join(workdir, ".paracell", "state.json")}

	switch cmd.Kind {
	case CommandVersion:
		_, err := os.Stdout.WriteString(formatVersion())
		return err
	case CommandHelp:
		_, err := os.Stdout.WriteString(usage)
		return err
	case CommandInit:
		uc := usecase.InitProjectUseCase{Config: configAdapter}
		_, err := uc.Execute(ctx)
		return err
	case CommandList:
		uc := usecase.ListCellsUseCase{State: stateAdapter}
		cells, err := uc.Execute(ctx)
		if err != nil {
			return err
		}
		_, err = os.Stdout.WriteString(output.FormatCellList(cells))
		return err
	case CommandPending:
		cellName, err := currentCellFromEnv()
		if err != nil {
			return err
		}
		_, err = runSetStatus(ctx, stateAdapter, cellName, domain.Pending)
		return err
	case CommandReady:
		cellName, err := currentCellFromEnv()
		if err != nil {
			return err
		}
		_, err = runSetStatus(ctx, stateAdapter, cellName, domain.Ready)
		return err
	case CommandView:
		uc := usecase.ViewCellsUseCase{State: stateAdapter}
		cells, err := uc.Execute(ctx)
		if err != nil {
			return err
		}
		_, err = runView(ctx, cells, func() ([]domain.Cell, error) {
			return stateAdapter.LoadCells(ctx)
		}, func(cell domain.Cell) tea.Cmd {
			cmd, err := runEnterCmd(ctx, configAdapter, cell)
			if err != nil {
				return viewadapter.EnterFailureCmd(cell, err)
			}
			return viewadapter.EnterProcessCmd(cell, cmd)
		}, func() error {
			return runExit(ctx, runner)
		}, func(cell domain.Cell) error {
			return runClean(ctx, configAdapter, provider.Factory{Runner: quietRunner, Root: workdir}, provider.Factory{Runner: quietRunner, Root: workdir}, provider.Factory{Runner: quietRunner, Root: workdir}, stateAdapter, cell)
		}, func(cell domain.Cell) (domain.Cell, error) {
			return runMarkDone(ctx, stateAdapter, cell)
		})
		if err != nil {
			return err
		}
		return nil
	case CommandFork:
		uc := usecase.ForkCellUseCase{
			Config:           configAdapter,
			State:            stateAdapter,
			CellFactory:      celladapter.Factory{},
			SourceFactory:    provider.Factory{Runner: runner, Root: workdir},
			Files:            files.CopyAdapter{Root: workdir},
			ContainerFactory: provider.Factory{Runner: runner, Root: workdir},
			SessionFactory:   provider.Factory{Runner: runner, Root: workdir},
			IDs:              id.RandomGenerator{},
		}
		_, err = uc.Execute(ctx, usecase.ForkCellInput{Issue: cmd.Issue, Template: cmd.Template})
		return err
	case CommandClean:
		uc := usecase.CleanCellUseCase{
			Config:           configAdapter,
			State:            stateAdapter,
			SourceFactory:    provider.Factory{Runner: runner, Root: workdir},
			ContainerFactory: provider.Factory{Runner: runner, Root: workdir},
			SessionFactory:   provider.Factory{Runner: runner, Root: workdir},
		}
		return uc.Execute(ctx, usecase.CleanCellInput{Cell: cmd.Cell})
	default:
		return fmt.Errorf("unsupported command %q", cmd.Kind)
	}
}

func formatVersion() string {
	return fmt.Sprintf("%s %s\ncommit: %s\nbuilt: %s\n", AppName, Version, Commit, BuildDate)
}

func currentCellFromEnv() (string, error) {
	cell := os.Getenv("PARACELL_CELL")
	if cell == "" {
		return "", errors.New("PARACELL_CELL is required")
	}
	return cell, nil
}

func projectRootForWorkdir(workdir string) string {
	if root := os.Getenv("PARACELL_ROOT"); root != "" {
		if abs, err := filepath.Abs(root); err == nil {
			return abs
		}
		return filepath.Clean(root)
	}
	for dir := filepath.Clean(workdir); ; dir = filepath.Dir(dir) {
		if filepath.Base(dir) == "source" {
			cellDir := filepath.Dir(dir)
			cellsDir := filepath.Dir(cellDir)
			paracellDir := filepath.Dir(cellsDir)
			if filepath.Base(cellsDir) == "cells" && filepath.Base(paracellDir) == ".paracell" {
				return filepath.Dir(paracellDir)
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return workdir
		}
	}
}
