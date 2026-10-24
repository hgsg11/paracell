package domain

import "fmt"

// CellGroupFactory rebuilds persisted CellGroups from raw values.
type CellGroupFactory struct{}

// Restore validates the raw persisted values and rebuilds a CellGroup, including
// its normalized note and creation state.
func (CellGroupFactory) Restore(id, issue, project, template, note, sourceDriver, containerDriver, notificationDriver, creationStatus, failedStage, lastError string) (CellGroup, error) {
	group, err := NewCellGroup(id, issue, project, template, SourceDriverType(sourceDriver), ContainerDriverType(containerDriver), NotificationDriverType(notificationDriver))
	if err != nil {
		return CellGroup{}, err
	}
	if note != "" {
		if err := group.SetNote(note); err != nil {
			return CellGroup{}, err
		}
	}
	status, err := NewCreationStatus(creationStatus)
	if err != nil {
		return CellGroup{}, err
	}
	// A failed creation always records its stage. Other states keep whatever was
	// persisted after validating it, as the previous restore did.
	if status == CreationFailed || failedStage != "" {
		if _, err := NewCreationStage(failedStage); err != nil {
			return CellGroup{}, fmt.Errorf("restore CellGroup %q creation: %w", id, err)
		}
	}
	group.Creation = CellCreation{Status: status, FailedStage: CreationStage(failedStage), LastError: lastError}
	return group, nil
}
