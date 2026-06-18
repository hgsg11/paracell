package session

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/hgsg11/paracell/internal/adapter/system"
	"github.com/hgsg11/paracell/internal/domain"
)

type TmuxAdapter struct {
	Runner system.Runner
}

func (a TmuxAdapter) CreateSession(ctx context.Context, cell domain.Cell) error {
	if len(cell.Session.Windows) == 0 {
		if err := a.Runner.Run(ctx, "tmux", "new-session", "-d", "-s", cell.Session.Name, "-e", "PARACELL_CELL="+cell.Name, "-c", cell.Source.Path); err != nil {
			return err
		}
		return a.configureSessionBindings(ctx, cell)
	}
	first := cell.Session.Windows[0]
	if err := a.Runner.Run(ctx, "tmux", "new-session", "-d", "-s", cell.Session.Name, "-e", "PARACELL_CELL="+cell.Name, "-n", first.Name, "-c", cell.Source.Path); err != nil {
		return err
	}
	if err := a.runWindowCommand(ctx, cell, first); err != nil {
		return err
	}
	for _, window := range cell.Session.Windows[1:] {
		if err := a.Runner.Run(ctx, "tmux", "new-window", "-t", cell.Session.Name, "-n", window.Name, "-c", cell.Source.Path); err != nil {
			return err
		}
		if err := a.runWindowCommand(ctx, cell, window); err != nil {
			return err
		}
	}
	return a.configureSessionBindings(ctx, cell)
}

func (a TmuxAdapter) runWindowCommand(ctx context.Context, cell domain.Cell, window domain.SessionWindow) error {
	if window.Command == "" {
		return nil
	}
	return a.Runner.Run(ctx, "tmux", "send-keys", "-t", cell.Session.Name+":"+window.Name, window.Command, "Enter")
}

func (a TmuxAdapter) configureSessionBindings(ctx context.Context, cell domain.Cell) error {
	if err := a.Runner.Run(ctx, "tmux", "set-option", "-t", cell.Session.Name, "key-table", "paracell"); err != nil {
		return err
	}
	return a.Runner.Run(ctx, "tmux", "bind-key", "-T", "paracell", "C-t", "next-window")
}

func (a TmuxAdapter) CleanSession(ctx context.Context, cell domain.Cell) error {
	err := a.Runner.Run(ctx, "tmux", "kill-session", "-t", cell.Session.Name)
	if err == nil {
		return nil
	}
	if strings.Contains(strings.ToLower(err.Error()), "can't find session") {
		return fmt.Errorf("%w: %v", domain.ErrNotFound, err)
	}
	return err
}

func (a TmuxAdapter) EnterSession(ctx context.Context, cell domain.Cell) error {
	if os.Getenv("TMUX") != "" {
		return a.Runner.Run(ctx, "tmux", "switch-client", "-t", cell.Session.Name)
	}
	return a.Runner.Run(ctx, "tmux", "attach-session", "-t", cell.Session.Name)
}
