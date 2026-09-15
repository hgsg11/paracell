package domain

import "context"

type SourcePort interface {
	CreateSource(context.Context, SourceResource) (bool, error)
	ResumeSource(context.Context, SourceResource) error
	CleanSource(context.Context, SourceResource) error
}
