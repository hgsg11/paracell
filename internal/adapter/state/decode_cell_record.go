package state

import (
	"encoding/json"
	"fmt"
)

// earlierCellRecord captures the keys of the two formats written before
// cellRecord. Both kept everything under "commander" and embedded the row
// version there:
//   - the previous format nested "cellGroup" (Go field names) and "creation"
//     inside "commander";
//   - the format before it had no CellGroup: the issue, project, note, template,
//     drivers, and member Cell IDs were on "commander", and each member Cell named
//     its CommanderID.
//
// JSON keys match case-insensitively, so the Go field names written then are read
// by these tags.
type earlierCellRecord struct {
	Commander struct {
		Version   uint64 `json:"version"`
		CellGroup *struct {
			ID                 string `json:"id"`
			Note               string `json:"note"`
			Issue              string `json:"issue"`
			Project            string `json:"project"`
			Template           string `json:"template"`
			NotificationDriver string `json:"notificationDriver"`
			SourceDriver       string `json:"sourceDriver"`
			ContainerDriver    string `json:"containerDriver"`
		} `json:"cellGroup"`
		Creation           creationRecord `json:"creation"`
		Issue              string         `json:"issue"`
		Project            string         `json:"project"`
		Note               string         `json:"note"`
		Template           string         `json:"template"`
		SourceDriver       string         `json:"sourceDriver"`
		ContainerDriver    string         `json:"containerDriver"`
		NotificationDriver string         `json:"notificationDriver"`
		Targets            []string       `json:"targets"`
		Dependencies       []string       `json:"dependencies"`
	} `json:"commander"`
	Targets []struct {
		CommanderID string           `json:"commanderId"`
		Container   *containerRecord `json:"container"`
	} `json:"targets"`
	Dependencies []struct {
		CommanderID string `json:"commanderId"`
	} `json:"dependencies"`
}

// decodeCellRecord reads the current record format and the two earlier formats
// into a cellRecord of raw values. version is the row's version column; an
// earlier format must embed the same value. Earlier rows are rewritten in the
// current format the next time their CellGroup or Cells change.
func decodeCellRecord(data []byte, version uint64) (cellRecord, error) {
	var record cellRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return cellRecord{}, err
	}
	if record.CellGroup != nil {
		return record, nil
	}
	var earlier earlierCellRecord
	if err := json.Unmarshal(data, &earlier); err != nil {
		return cellRecord{}, err
	}
	if earlier.Commander.Version != version {
		return cellRecord{}, fmt.Errorf("CommanderCell %q version %d does not match its state row version %d", record.Commander.ID, earlier.Commander.Version, version)
	}
	// The format before CellGroup stored one container per TargetCell.
	for i := range record.Targets {
		if len(record.Targets[i].Containers) == 0 && i < len(earlier.Targets) && earlier.Targets[i].Container != nil {
			record.Targets[i].Containers = []containerRecord{*earlier.Targets[i].Container}
		}
	}
	old := earlier.Commander
	if old.CellGroup != nil {
		raw := old.CellGroup
		record.CellGroup = &cellGroupRecord{
			ID: raw.ID, Note: raw.Note, Issue: raw.Issue, Project: raw.Project, Template: raw.Template,
			NotificationDriver: raw.NotificationDriver, SourceDriver: raw.SourceDriver, ContainerDriver: raw.ContainerDriver,
			Creation: old.Creation,
		}
		record.Commander.CellGroupID = raw.ID
		return record, nil
	}
	return decodeRecordWithoutCellGroup(record, earlier)
}

// decodeRecordWithoutCellGroup converts the format before CellGroup. The
// CellGroup takes the CommanderCell ID, and the legacy membership lists are
// checked before they are replaced with CellGroup references.
func decodeRecordWithoutCellGroup(record cellRecord, earlier earlierCellRecord) (cellRecord, error) {
	old := earlier.Commander
	groupID := record.Commander.ID
	record.CellGroup = &cellGroupRecord{
		ID: groupID, Note: old.Note, Issue: old.Issue, Project: old.Project, Template: old.Template,
		NotificationDriver: old.NotificationDriver, SourceDriver: old.SourceDriver, ContainerDriver: old.ContainerDriver,
		Creation: old.Creation,
	}
	record.Commander.CellGroupID = groupID
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
		if i >= len(earlier.Targets) || earlier.Targets[i].CommanderID != record.Commander.ID || !targetIDs[target.ID] {
			return cellRecord{}, fmt.Errorf("invalid legacy TargetCell membership %q", target.ID)
		}
		delete(targetIDs, target.ID)
		target.CellGroupID = groupID
	}
	for i := range record.Dependencies {
		dependency := &record.Dependencies[i]
		if i >= len(earlier.Dependencies) || earlier.Dependencies[i].CommanderID != record.Commander.ID || !dependencyIDs[dependency.ID] {
			return cellRecord{}, fmt.Errorf("invalid legacy DependencyCell membership %q", dependency.ID)
		}
		delete(dependencyIDs, dependency.ID)
		dependency.CellGroupID = groupID
	}
	if len(targetIDs) != 0 || len(dependencyIDs) != 0 {
		return cellRecord{}, fmt.Errorf("legacy CommanderCell references missing Cells")
	}
	return record, nil
}
