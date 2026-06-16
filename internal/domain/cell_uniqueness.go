package domain

import "fmt"

type CellUniquenessChecker struct{}

func (c CellUniquenessChecker) EnsureUnique(existing []Cell, issue string, name string) error {
	for _, cell := range existing {
		if cell.Issue == issue {
			return fmt.Errorf("cell issue %q already exists", issue)
		}
		if cell.Name == name {
			return fmt.Errorf("cell name %q already exists", name)
		}
	}
	return nil
}
