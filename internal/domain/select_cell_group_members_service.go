package domain

// SelectCellGroupMembersService resolves membership from the Cells, not a duplicate
// list on CommanderCell or CellGroup. It preserves their persisted order.
func SelectCellGroupMembersService(groupID string, targets []TargetCell, dependencies []DependencyCell) ([]TargetCell, []DependencyCell) {
	var selectedTargets []TargetCell
	for _, target := range targets {
		if target.CellGroupID == groupID {
			selectedTargets = append(selectedTargets, target)
		}
	}
	var selectedDependencies []DependencyCell
	for _, dependency := range dependencies {
		if dependency.CellGroupID == groupID {
			selectedDependencies = append(selectedDependencies, dependency)
		}
	}
	return selectedTargets, selectedDependencies
}
