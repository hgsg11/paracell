package domain

import "context"

type SourcePort interface {
	CreateSource(context.Context, SourceResource) error
	CleanSource(context.Context, SourceResource) error
}
