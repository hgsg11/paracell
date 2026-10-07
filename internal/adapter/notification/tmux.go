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
	if err := n.Runner.Run(ctx, "tmux", args...); err != nil {
		return err
	}
	return n.notifyClients(ctx, sessionName, message)
}

func (n TmuxNotifier) notifyClients(ctx context.Context, sessionName string, message string) error {
	clients, err := n.Runner.Output(ctx, "tmux", "list-clients", "-t", sessionName, "-F", "#{client_tty}")
	if err != nil {
		return err
	}
	message = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, message)
	sequence := "\x1b]9;" + message + "\x07"
	for _, tty := range strings.Split(strings.TrimSpace(clients), "\n") {
		tty = strings.TrimSpace(tty)
		if tty == "" {
			continue
		}
		command := "printf '%s' " + shellQuote(sequence) + " > " + shellQuote(tty)
		if err := n.Runner.Run(ctx, "tmux", "run-shell", "-t", sessionName, command); err != nil {
			return err
		}
	}
	return nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
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
