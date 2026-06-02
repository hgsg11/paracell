package system

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type CaptureRunner struct {
	Dir string
}

func (r CaptureRunner) Run(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = r.Dir
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return nil
	}
	output := strings.TrimSpace(stderr.String())
	if output == "" {
		output = strings.TrimSpace(stdout.String())
	}
	if output == "" {
		return err
	}
	return fmt.Errorf("%w: %s", err, output)
}

func (r CaptureRunner) Output(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = r.Dir
	data, err := cmd.Output()
	return strings.TrimSpace(string(data)), err
}
