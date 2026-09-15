package notification

import "context"

type NoopNotifier struct{}

func NewNoopNotifier() NoopNotifier { return NoopNotifier{} }

func (NoopNotifier) NotifyReady(ctx context.Context, sessionName string, message string) error {
	_ = ctx
	_ = sessionName
	_ = message
	return nil
}
