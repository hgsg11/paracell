package notification

import (
	"context"

	"github.com/hgsg11/paracell/internal/domain"
)

type NoopNotifier struct{}

func (NoopNotifier) NotifyReady(ctx context.Context, cell domain.Cell, message string) error {
	_ = ctx
	_ = cell
	_ = message
	return nil
}
