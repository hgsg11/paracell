package notification

import (
	"context"
	"errors"
	"strings"

	"github.com/hgsg11/paracell/internal/adapter/system"
	"github.com/hgsg11/paracell/internal/domain"
)

type TmuxNotifier struct {
	Runner system.Runner
}

func NewTmuxNotifier(runner system.Runner) TmuxNotifier { return TmuxNotifier{Runner: runner} }

func (n TmuxNotifier) NotifyReady(ctx context.Context, resource domain.WorkspaceResource, message string) error {
	if message == "" {
		return nil
	}
	// Cells live on the project's dedicated server; cells created before it
	// existed are still on the default server.
	socket := resource.SocketName()
	clients, err := n.listClients(ctx, socket, resource.Name)
	if err != nil {
		legacyClients, legacyErr := n.listClients(ctx, "", resource.Name)
		if legacyErr != nil {
			return errors.Join(err, legacyErr)
		}
		socket, clients = "", legacyClients
	}
	for _, client := range strings.Fields(clients) {
		if err := n.Runner.Run(ctx, "tmux", withSocket(socket, "display-message", "-c", client, message)...); err != nil {
			return err
		}
	}
	return nil
}

func (n TmuxNotifier) listClients(ctx context.Context, socket string, sessionName string) (string, error) {
	return n.Runner.Output(ctx, "tmux", withSocket(socket, "list-clients", "-t", sessionName, "-F", "#{client_tty}")...)
}

func withSocket(socket string, args ...string) []string {
	if socket == "" {
		return args
	}
	return append([]string{"-L", socket}, args...)
}
