package system

import (
	"context"
)

type Runner interface {
	Run(ctx context.Context, name string, args ...string) error
	Output(ctx context.Context, name string, args ...string) (string, error)
}
