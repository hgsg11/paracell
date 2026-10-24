package domain

// ResolveCommanderCellService finds the CommanderCell of the first CellGroup, in
// list order, that matches identifier. It reports false when no CellGroup matches
// or the matching CellGroup has no CommanderCell.
func ResolveCommanderCellService(groups []CellGroup, commanders []CommanderCell, identifier string) (CommanderCell, bool) {
	for _, group := range groups {
		if !group.Matches(identifier) {
			continue
		}
		for _, commander := range commanders {
			if commander.CellGroupID == group.ID {
				return commander, true
			}
		}
		return CommanderCell{}, false
	}
	return CommanderCell{}, false
}
