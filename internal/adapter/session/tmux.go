package session

import (
	"context"
	"os"

	"github.com/shige1114/paradev/internal/adapter/system"
	"github.com/shige1114/paradev/internal/domain"
)

type TmuxAdapter struct {
	Runner system.Runner
}

func (a TmuxAdapter) CreateSession(ctx context.Context, cell domain.Cell) error {
	return a.Runner.Run(ctx, "tmux", "new-session", "-d", "-s", cell.Session.Name, "-c", cell.Source.Path)
}

func (a TmuxAdapter) RemoveSession(ctx context.Context, cell domain.Cell) error {
	return a.Runner.Run(ctx, "tmux", "kill-session", "-t", cell.Session.Name)
}

func (a TmuxAdapter) EnterSession(ctx context.Context, cell domain.Cell) error {
	if os.Getenv("TMUX") != "" {
		return a.Runner.Run(ctx, "tmux", "switch-client", "-t", cell.Session.Name)
	}
	return a.Runner.Run(ctx, "tmux", "attach-session", "-t", cell.Session.Name)
}
