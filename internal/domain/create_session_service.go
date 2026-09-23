package domain

import (
	"context"
	"errors"
	"fmt"
)

type SessionCreationPort interface {
	CreateSession(ctx context.Context, name string, cellName string, firstWindow string, workingDirectory string) error
	CreateWindow(ctx context.Context, session string, window string, workingDirectory string) error
	SendWindowCommand(ctx context.Context, session string, window string, command string) error
	ConfigureSession(ctx context.Context, name string, cellName string, project string, label string, windowNames []string) error
	CleanSession(ctx context.Context, name string) error
}

func CreateSessionService(ctx context.Context, template SessionTemplate, name string, cellName string, project string, label string, workingDirectory string, port SessionCreationPort) (returnErr error) {
	windowNames := make([]string, 0, len(template.Windows))
	for _, window := range template.Windows {
		windowNames = append(windowNames, window.Name)
	}
	firstWindow := ""
	if len(template.Windows) > 0 {
		firstWindow = template.Windows[0].Name
	}
	if err := port.CreateSession(ctx, name, cellName, firstWindow, workingDirectory); err != nil {
		return err
	}
	defer func() {
		if returnErr == nil {
			return
		}
		if err := port.CleanSession(context.WithoutCancel(ctx), name); err != nil && !errors.Is(err, ErrNotFound) {
			returnErr = errors.Join(returnErr, fmt.Errorf("clean partial session: %w", err))
		}
	}()
	for index, window := range template.Windows {
		if index > 0 {
			if err := port.CreateWindow(ctx, name, window.Name, workingDirectory); err != nil {
				return err
			}
		}
		if window.Command != "" {
			if err := port.SendWindowCommand(ctx, name, window.Name, window.Command); err != nil {
				return err
			}
		}
	}
	return port.ConfigureSession(ctx, name, cellName, project, label, windowNames)
}
