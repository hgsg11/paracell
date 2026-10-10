package state

import (
	"encoding/json"
	"fmt"

	"github.com/hgsg11/paracell/internal/domain"
)

// decodeCellRecord migrates Commander-owned persisted groups to the independent
// CellGroup member while preserving legacy target and dependency membership.
func decodeCellRecord(data []byte) (stateCellRecord, error) {
	var record stateCellRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return stateCellRecord{}, err
	}
	var legacy struct {
		Commander struct {
			CellGroup          *domain.CellGroup
			Creation           domain.CellCreation
			Issue              string
			Project            string
			Note               string
			Template           string
			SourceDriver       domain.SourceDriverType
			ContainerDriver    domain.ContainerDriverType
			NotificationDriver domain.NotificationDriverType
			Targets            []string
			Dependencies       []string
		}
		Targets      []struct{ CommanderID string }
		Dependencies []struct{ CommanderID string }
	}
	if err := json.Unmarshal(data, &legacy); err != nil {
		return stateCellRecord{}, err
	}
	var oldTargets struct {
		Targets []struct{ Container *domain.Container }
	}
	if err := json.Unmarshal(data, &oldTargets); err != nil {
		return stateCellRecord{}, err
	}
	for i := range record.Targets {
		if len(record.Targets[i].Containers) == 0 && i < len(oldTargets.Targets) && oldTargets.Targets[i].Container != nil {
			container := *oldTargets.Targets[i].Container
			record.Targets[i].Containers = []*domain.Container{&container}
		}
	}
	if record.Group.ID != "" {
		return record, nil
	}
	if legacy.Commander.CellGroup != nil {
		group, err := domain.RestoreCellGroup(*legacy.Commander.CellGroup)
		if err != nil {
			return stateCellRecord{}, err
		}
		if legacy.Commander.Creation.Status != "" {
			group.Creation = legacy.Commander.Creation
		}
		record.Group = group
		record.Commander.CellGroupID = group.ID
		return record, nil
	}

	old := legacy.Commander
	group, err := domain.NewCellGroup(record.Commander.ID, old.Issue, old.Project, old.Template, old.SourceDriver, old.ContainerDriver, old.NotificationDriver)
	if err != nil {
		return stateCellRecord{}, err
	}
	if old.Note != "" {
		if err := group.SetNote(old.Note); err != nil {
			return stateCellRecord{}, err
		}
	}
	if old.Creation.Status != "" {
		group.Creation = old.Creation
	}
	targetIDs := make(map[string]bool, len(old.Targets))
	for _, id := range old.Targets {
		targetIDs[id] = true
	}
	dependencyIDs := make(map[string]bool, len(old.Dependencies))
	for _, id := range old.Dependencies {
		dependencyIDs[id] = true
	}
	for i := range record.Targets {
		target := &record.Targets[i]
		if i >= len(legacy.Targets) || legacy.Targets[i].CommanderID != record.Commander.ID || !targetIDs[target.ID] {
			return stateCellRecord{}, fmt.Errorf("invalid legacy TargetCell membership %q", target.ID)
		}
		delete(targetIDs, target.ID)
		target.CellGroupID = group.ID
	}
	for i := range record.Dependencies {
		dependency := &record.Dependencies[i]
		if i >= len(legacy.Dependencies) || legacy.Dependencies[i].CommanderID != record.Commander.ID || !dependencyIDs[dependency.ID] {
			return stateCellRecord{}, fmt.Errorf("invalid legacy DependencyCell membership %q", dependency.ID)
		}
		delete(dependencyIDs, dependency.ID)
		dependency.CellGroupID = group.ID
	}
	if len(targetIDs) != 0 || len(dependencyIDs) != 0 {
		return stateCellRecord{}, fmt.Errorf("legacy CommanderCell references missing Cells")
	}
	record.Group = group
	record.Commander.CellGroupID = group.ID
	return record, nil
}
