package usecase

import (
	"context"

	"github.com/hgsg11/paracell/internal/domain"
)

type MarkCellReadyInput struct {
	Cell string
}

// MarkCellReadyUseCase backs `paracell ready`. After saving, it notifies through
// the CellGroup's notification driver when a NotificationFactory is set.
type MarkCellReadyUseCase struct {
	Cells               CellPort
	NotificationFactory NotificationProviderFactory
}

func (u MarkCellReadyUseCase) Execute(ctx context.Context, input MarkCellReadyInput) (domain.CommanderCell, error) {
	commander, group, err := updateCommander(ctx, u.Cells, input.Cell, func(commander *domain.CommanderCell) error {
		commander.MarkReady()
		return nil
	})
	if err != nil {
		return domain.CommanderCell{}, err
	}
	if u.NotificationFactory == nil {
		return commander, nil
	}
	notifier, err := u.NotificationFactory.Notification(group.ResourceDrivers(commander.Workspace.Driver).Notification)
	if err != nil {
		return domain.CommanderCell{}, err
	}
	if err := notifier.NotifyReady(ctx, group.WorkspaceResource(commander.Workspace), "Ready: "+group.Name().Value); err != nil {
		return domain.CommanderCell{}, err
	}
	return commander, nil
}
