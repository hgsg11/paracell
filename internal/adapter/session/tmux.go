package session

import (
	"context"

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
