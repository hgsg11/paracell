package system

import (
	"context"
)

type Runner interface {
	Run(ctx context.Context, name string, args ...string) error
	Output(ctx context.Context, name string, args ...string) (string, error)
}

// DirRunner is a Runner that can run commands from another directory, for
// tools such as gh that read the repository from the working directory.
type DirRunner interface {
	Runner
	InDir(dir string) Runner
}
