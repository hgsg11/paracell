package domain

import "fmt"

func EnsureUniqueCellGroupIssueService(groups []CellGroup, issue string) error {
	for _, group := range groups {
		if group.Issue == issue {
			return fmt.Errorf("commander cell issue %q already exists", issue)
		}
	}
	return nil
}
