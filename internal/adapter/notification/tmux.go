package notification

import (
	"context"
	"os"
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
	args := []string{"display-message"}
	if client := n.targetClient(ctx, sessionName); client != "" {
		args = append(args, "-c", client)
	}
	args = append(args, message)
	return n.Runner.Run(ctx, "tmux", args...)
}

func (n TmuxNotifier) targetClient(ctx context.Context, sessionName string) string {
	if os.Getenv("TMUX") != "" {
		client, err := n.Runner.Output(ctx, "tmux", "display-message", "-p", "#{client_tty}")
		if err == nil {
			return strings.TrimSpace(client)
		}
	}
	client, err := n.Runner.Output(ctx, "tmux", "list-clients", "-t", sessionName, "-F", "#{client_tty}")
	if err != nil {
		return ""
	}
	lines := strings.Split(client, "\n")
	if len(lines) == 0 {
		return ""
	}
	return strings.TrimSpace(lines[0])
}
