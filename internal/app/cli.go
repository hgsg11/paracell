package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/shige1114/paradev/internal/adapter/config"
	"github.com/shige1114/paradev/internal/adapter/id"
	"github.com/shige1114/paradev/internal/adapter/output"
	"github.com/shige1114/paradev/internal/adapter/state"
	"github.com/shige1114/paradev/internal/adapter/system"
	"github.com/shige1114/paradev/internal/usecase"
)

type CommandKind string

const (
	CommandInit   CommandKind = "init"
	CommandCreate CommandKind = "create"
	CommandRemove CommandKind = "remove"
	CommandList   CommandKind = "ls"
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
		return Command{}, errors.New("command is required")
	}
	switch args[0] {
	case "init":
		if len(args) != 1 {
			return Command{}, errors.New("usage: pdev init")
		}
		return Command{Kind: CommandInit}, nil
	case "ls":
		if len(args) != 1 {
			return Command{}, errors.New("usage: pdev ls")
		}
		return Command{Kind: CommandList}, nil
	case "create":
		if len(args) != 4 || args[2] != "--template" || args[1] == "" || args[3] == "" {
			return Command{}, errors.New("usage: pdev create <issue> --template <template>")
		}
		return Command{Kind: CommandCreate, Issue: args[1], Template: args[3]}, nil
	case "remove":
		if len(args) != 2 && !(len(args) == 3 && args[2] == "--force") {
			return Command{}, errors.New("usage: pdev remove <cell> [--force]")
		}
		return Command{Kind: CommandRemove, Cell: args[1], Force: len(args) == 3}, nil
	default:
		return Command{}, fmt.Errorf("unsupported command %q", args[0])
	}
}

func Run(ctx context.Context, args []string, workdir string) error {
	cmd, err := ParseCommand(args)
	if err != nil {
		return err
	}
	runner := system.OSCommandRunner{Dir: workdir}
	configAdapter := config.YAMLConfigAdapter{Path: filepath.Join(workdir, ".pdev.yml")}
	stateAdapter := state.JSONCellStateAdapter{Path: filepath.Join(workdir, ".pdev", "state.json")}

	switch cmd.Kind {
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
	case CommandCreate:
		cfg, err := configAdapter.Load(ctx)
		if err != nil {
			return err
		}
		adapters, err := NewProviderAdapters(cfg.Providers, runner)
		if err != nil {
			return err
		}
		uc := usecase.CreateCellUseCase{
			Config:     staticConfig{cfg: cfg},
			State:      stateAdapter,
			Source:     adapters.Source,
			Containers: adapters.Containers,
			Session:    adapters.Session,
			IDs:        id.RandomGenerator{},
		}
		_, err = uc.Execute(ctx, usecase.CreateCellInput{Issue: cmd.Issue, Template: cmd.Template})
		return err
	case CommandRemove:
		cfg, err := configAdapter.Load(ctx)
		if err != nil {
			return err
		}
		adapters, err := NewProviderAdapters(cfg.Providers, runner)
		if err != nil {
			return err
		}
		uc := usecase.RemoveCellUseCase{
			State:      stateAdapter,
			Source:     adapters.Source,
			Containers: adapters.Containers,
			Session:    adapters.Session,
		}
		return uc.Execute(ctx, usecase.RemoveCellInput{Cell: cmd.Cell})
	default:
		return fmt.Errorf("unsupported command %q", cmd.Kind)
	}
}
