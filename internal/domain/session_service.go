package domain

import "context"

func CreateSession(ctx context.Context, cell *Cell, driver SessionDriverType, template SessionTemplate, port SessionPort) error {
	session, err := buildSession(driver, template)
	if err != nil {
		return err
	}
	cell.PlanSession(session)
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
