package domain

import "context"

func CreateSessionService(ctx context.Context, cell Cell, template SessionTemplate, create func(context.Context, SessionTemplate, string, string, string, string, string) error) error {
	return create(ctx, template, cell.SessionName(), cell.Name().Value, cell.Project, cell.DisplayLabel(), cell.WorkingDirectory())
}
