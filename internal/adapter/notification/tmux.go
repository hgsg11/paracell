package notification

import (
	"context"
	"strings"

	"github.com/hgsg11/paracell/internal/adapter/system"
)

type TmuxNotifier struct {
	Runner system.Runner
}

func NewTmuxNotifier(runner system.Runner) TmuxNotifier { return TmuxNotifier{Runner: runner} }

func (n TmuxNotifier) NotifyReady(ctx context.Context, sessionName string, message string) error {
	if message == "" {
		return nil
	}
	clients, err := n.Runner.Output(ctx, "tmux", "list-clients", "-t", sessionName, "-F", "#{client_tty}")
	if err != nil {
		return err
	}
	for _, client := range strings.Fields(clients) {
		if err := n.Runner.Run(ctx, "tmux", "display-message", "-c", client, message); err != nil {
			return err
		}
	}
	return nil
}
