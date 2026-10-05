package state

import (
	"encoding/json"
	"fmt"

	"github.com/hgsg11/paracell/internal/domain"
)

// decodeCellRecord reads the current record and the preceding Commander-owned
// format. Legacy membership is checked before replacing it with CellGroup IDs;
// a normal transactional update subsequently writes only the current format.
func decodeCellRecord(data []byte) (stateCellRecord, error) {
	var record stateCellRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return stateCellRecord{}, err
	}
	var oldTargets struct {
		Targets []struct {
			Container *domain.Container
		}
	}
	if err := json.Unmarshal(data, &oldTargets); err != nil {
		return stateCellRecord{}, err
	}
	for i := range record.Targets {
		if len(record.Targets[i].Containers) == 0 && i < len(oldTargets.Targets) && oldTargets.Targets[i].Container != nil {
			record.Targets[i].Containers = []domain.Container{*oldTargets.Targets[i].Container}
		}
	}
	if record.Commander.CellGroup != nil {
		return record, nil
	}
	var legacy struct {
		Commander struct {
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
		if legacy.Targets[i].CommanderID != record.Commander.ID || !targetIDs[target.ID] {
			return stateCellRecord{}, fmt.Errorf("invalid legacy TargetCell membership %q", target.ID)
		}
		delete(targetIDs, target.ID)
		target.CellGroupID = group.ID
	}
	for i := range record.Dependencies {
		dependency := &record.Dependencies[i]
		if legacy.Dependencies[i].CommanderID != record.Commander.ID || !dependencyIDs[dependency.ID] {
			return stateCellRecord{}, fmt.Errorf("invalid legacy DependencyCell membership %q", dependency.ID)
		}
		delete(dependencyIDs, dependency.ID)
		dependency.CellGroupID = group.ID
	}
	if len(targetIDs) != 0 || len(dependencyIDs) != 0 {
		return stateCellRecord{}, fmt.Errorf("legacy CommanderCell references missing Cells")
	}
	record.Commander.CellGroup = &group
	return record, nil
}
