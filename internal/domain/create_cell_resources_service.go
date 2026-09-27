package domain

import "context"

type CellSourceCreationPort interface {
	CreateSource(ctx context.Context, repository string, worktree string, base string, branch string) error
}

type CellContainerCreationPort interface {
	CreateContainerNetwork(ctx context.Context, network string) error
	CreateContainer(ctx context.Context, containerName string, mode Mode, environments []Environment, mounts []Mount, cellName string, project string, network string, sourcePath string) ([]string, error)
}

type CellSessionCreationPort interface {
	CreateSession(ctx context.Context, name string, cellName string, firstWindow string, workingDirectory string) error
	CreateWindow(ctx context.Context, session string, window string, workingDirectory string) error
	SendWindowCommand(ctx context.Context, session string, window string, command string) error
	ConfigureSession(ctx context.Context, name string, cellName string, project string, label string, windowNames []string) error
}

func CreateCellResourcesService(ctx context.Context, cell *Cell, sourcePort CellSourceCreationPort, containerPort CellContainerCreationPort, sessionPort CellSessionCreationPort) error {
	for _, source := range cell.Sources.Items {
		if err := sourcePort.CreateSource(ctx, source.Path, source.Worktree, source.Base, source.Branch); err != nil {
			return err
		}
	}

	name, cellName, project, label, windowNames := cell.SessionPreparation()
	network := cell.ContainerNetworkName()
	if err := containerPort.CreateContainerNetwork(ctx, network); err != nil {
		return err
	}
	networks := make(map[string][]string, len(cell.Containers.Items))
	for _, container := range cell.Containers.Items {
		containerNetworks, err := containerPort.CreateContainer(ctx, container.SourceContainer, container.Mode, container.Environments, container.Mounts, cellName, project, network, cell.WorkingDirectory())
		if err != nil {
			return err
		}
		networks[container.SourceContainer] = containerNetworks
	}
	cell.RecordContainerNetworks(networks)

	firstWindow := ""
	if len(cell.Session.Windows) > 0 {
		firstWindow = cell.Session.Windows[0].Name
	}
	if err := sessionPort.CreateSession(ctx, name, cellName, firstWindow, cell.WorkingDirectory()); err != nil {
		return err
	}
	for index, window := range cell.Session.Windows {
		if index > 0 {
			if err := sessionPort.CreateWindow(ctx, name, window.Name, cell.WorkingDirectory()); err != nil {
				return err
			}
		}
		if window.Command != "" {
			if err := sessionPort.SendWindowCommand(ctx, name, window.Name, window.Command); err != nil {
				return err
			}
		}
	}
	return sessionPort.ConfigureSession(ctx, name, cellName, project, label, windowNames)
}
