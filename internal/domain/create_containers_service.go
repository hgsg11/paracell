package domain

import "context"

func CreateContainersService(ctx context.Context, cell *Cell, templates []ContainerTemplate, create func(context.Context, []ContainerTemplate, string, string, string, string) (map[string][]string, error)) error {
	networks, err := create(ctx, templates, cell.Name().Value, cell.Project, cell.ContainerNetworkName(), cell.WorkingDirectory())
	if err != nil {
		return err
	}
	cell.RecordContainerNetworks(networks)
	return nil
}
