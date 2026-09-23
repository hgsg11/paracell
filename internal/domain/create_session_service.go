package domain

import "context"

type SessionCreationPort interface {
	CreateSession(ctx context.Context, template SessionTemplate, name string, cellName string, project string, label string, workingDirectory string) error
}

func CreateSessionService(ctx context.Context, template SessionTemplate, name string, cellName string, project string, label string, workingDirectory string, port SessionCreationPort) error {
	return port.CreateSession(ctx, template, name, cellName, project, label, workingDirectory)
}
