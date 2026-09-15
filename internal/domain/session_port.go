package domain

import "context"

type SessionPort interface {
	CreateSession(context.Context, SessionResource) error
	CleanSession(context.Context, SessionResource) error
	PrepareSession(context.Context, SessionResource) error
	UpdateStatusLabel(context.Context, SessionResource) error
	EnterSession(context.Context, SessionResource) error
	EnterRootSession(context.Context, string) error
	ExitSession(context.Context) error
}
