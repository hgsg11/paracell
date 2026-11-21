package domain

import (
	"path/filepath"
	"sort"
	"strings"
)

// BuildSourceResourcesService returns one SourceResource per source path, from
// the shallowest path to the deepest one, so a worktree nested in another
// worktree is created after it. Clean walks the result in reverse. TargetCells
// sharing a source path share its worktree, because the CellGroup has a single
// branch.
func BuildSourceResourcesService(group CellGroup, targets []TargetCell) []SourceResource {
	layout := BuildCellResourceLayoutService(group, targets)
	resources := make([]SourceResource, 0, len(targets))
	seen := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		if target.Source == nil {
			continue
		}
		if _, exists := seen[target.Source.Path]; exists {
			continue
		}
		seen[target.Source.Path] = struct{}{}
		resources = append(resources, NewSourceResource(target.Source.Path, layout.SourceWorktreePath(target.Name), target.Source.Base, target.Source.Branch))
	}
	sort.SliceStable(resources, func(i, j int) bool {
		left, right := pathDepth(resources[i].RepositoryPath), pathDepth(resources[j].RepositoryPath)
		if left != right {
			return left < right
		}
		return resources[i].RepositoryPath < resources[j].RepositoryPath
	})
	return resources
}

func pathDepth(path string) int {
	clean := filepath.Clean(path)
	if clean == "." {
		return 0
	}
	return strings.Count(clean, string(filepath.Separator)) + 1
}
