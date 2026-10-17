package notification

import (
	"context"

	"github.com/hgsg11/paracell/internal/adapter/system"
	"github.com/hgsg11/paracell/internal/domain"
)

type TerminalNotifier struct {
	Runner system.Runner
}

func NewTerminalNotifier(runner system.Runner) TerminalNotifier {
	return TerminalNotifier{Runner: runner}
}

func (n TerminalNotifier) NotifyReady(ctx context.Context, _ domain.WorkspaceResource, message string) error {
	return n.Runner.Run(ctx, "terminal-notifier", "-message", message)
}
