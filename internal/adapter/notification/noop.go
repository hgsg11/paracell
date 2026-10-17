package notification

import (
	"context"

	"github.com/hgsg11/paracell/internal/domain"
)

type NoopNotifier struct{}

func NewNoopNotifier() NoopNotifier { return NoopNotifier{} }

func (NoopNotifier) NotifyReady(ctx context.Context, resource domain.WorkspaceResource, message string) error {
	_ = ctx
	_ = resource
	_ = message
	return nil
}
