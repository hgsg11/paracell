package domain

import "fmt"

// ChangePlan is the set of files a queued Cell declares it will change: Modify
// lists files that exist (deletions included) and Create lists new files. The
// two lists stay separate because the enqueue rules differ by kind.
type ChangePlan struct {
	Modify []WorkspacePath
	Create []WorkspacePath
}

// NewChangePlan removes duplicates within each list. A path in both lists, or
// a plan with no path at all, is an error.
func NewChangePlan(modify []WorkspacePath, create []WorkspacePath) (ChangePlan, error) {
	plan := ChangePlan{Modify: uniquePaths(modify), Create: uniquePaths(create)}
	if len(plan.Modify) == 0 && len(plan.Create) == 0 {
		return ChangePlan{}, fmt.Errorf("change plan requires at least one modify or create path")
	}
	modified := NewExistingPaths(plan.Modify)
	for _, p := range plan.Create {
		if modified.Contains(p) {
			return ChangePlan{}, fmt.Errorf("path %q is declared as both modify and create", p.Value)
		}
	}
	return plan, nil
}

// Paths returns every declared path, modify first.
func (p ChangePlan) Paths() []WorkspacePath {
	return append(append([]WorkspacePath(nil), p.Modify...), p.Create...)
}

// Overlaps reports whether the two plans declare at least one common path,
// whatever its kind.
func (p ChangePlan) Overlaps(other ChangePlan) bool {
	paths := NewExistingPaths(p.Paths())
	for _, path := range other.Paths() {
		if paths.Contains(path) {
			return true
		}
	}
	return false
}

func uniquePaths(paths []WorkspacePath) []WorkspacePath {
	seen := make(map[WorkspacePath]struct{}, len(paths))
	var unique []WorkspacePath
	for _, p := range paths {
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		unique = append(unique, p)
	}
	return unique
}
