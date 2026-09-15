package domain

import "context"

func CreateSession(ctx context.Context, cell Cell, port SessionPort) error {
	return port.CreateSession(ctx, cell.sessionResource())
}

func CleanSession(ctx context.Context, cell Cell, port SessionPort) error {
	return port.CleanSession(ctx, cell.sessionResource())
}

func PrepareSession(ctx context.Context, cell Cell, port SessionPort) error {
	return port.PrepareSession(ctx, cell.sessionResource())
}

func UpdateSessionStatusLabel(ctx context.Context, cell Cell, port SessionPort) error {
	return port.UpdateStatusLabel(ctx, cell.sessionResource())
}

func EnterSession(ctx context.Context, cell Cell, port SessionPort) error {
	return port.EnterSession(ctx, cell.sessionResource())
}
