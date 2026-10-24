package domain

import "fmt"

// EnsureCommanderCellUniqueService rejects a new Cell whose issue or Cell name is
// already used by an existing CellGroup.
func EnsureCommanderCellUniqueService(groups []CellGroup, issue string, name CellName) error {
	for _, group := range groups {
		if group.Issue == issue {
			return fmt.Errorf("commander cell issue %q already exists", issue)
		}
		if group.Name() == name {
			return fmt.Errorf("commander cell name %q already exists", name.Value)
		}
	}
	return nil
}
