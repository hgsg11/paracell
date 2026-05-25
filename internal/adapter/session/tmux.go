package session

import (
	"context"
	"os"

	"github.com/hgsg11/paracell/internal/adapter/system"
	"github.com/hgsg11/paracell/internal/domain"
)

type TmuxAdapter struct {
	Runner system.Runner
}

func (a TmuxAdapter) CreateSession(ctx context.Context, cell domain.Cell) error {
	if len(cell.Session.Windows) == 0 {
		return a.Runner.Run(ctx, "tmux", "new-session", "-d", "-s", cell.Session.Name, "-c", cell.Source.Path)
	}
	first := cell.Session.Windows[0]
	if err := a.Runner.Run(ctx, "tmux", "new-session", "-d", "-s", cell.Session.Name, "-n", first.Name, "-c", cell.Source.Path); err != nil {
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
	return nil
}

func (a TmuxAdapter) runWindowCommand(ctx context.Context, cell domain.Cell, window domain.SessionWindow) error {
	if window.Command == "" {
		return nil
	}
	return a.Runner.Run(ctx, "tmux", "send-keys", "-t", cell.Session.Name+":"+window.Name, window.Command, "Enter")
}

func (a TmuxAdapter) CleanSession(ctx context.Context, cell domain.Cell) error {
	return a.Runner.Run(ctx, "tmux", "kill-session", "-t", cell.Session.Name)
}

func (a TmuxAdapter) EnterSession(ctx context.Context, cell domain.Cell) error {
	if os.Getenv("TMUX") != "" {
		return a.Runner.Run(ctx, "tmux", "switch-client", "-t", cell.Session.Name)
	}
	return a.Runner.Run(ctx, "tmux", "attach-session", "-t", cell.Session.Name)
}
