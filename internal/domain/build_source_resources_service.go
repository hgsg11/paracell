package domain

// BuildSourceResourcesService returns one SourceResource per source path.
// TargetCells sharing a source path share the owner's worktree and get a link
// to it, because the CellGroup has a single branch.
func BuildSourceResourcesService(group CellGroup, targets []TargetCell) []SourceResource {
	layout := BuildCellResourceLayoutService(group, targets)
	resources := make([]SourceResource, 0, len(targets))
	indexes := make(map[string]int, len(targets))
	for _, target := range targets {
		if target.Source == nil || layout.SourceOwner(target.Name) != target.Name {
			continue
		}
		indexes[target.Name] = len(resources)
		resources = append(resources, NewSourceResource(target.Source.Path, layout.SourceWorktreePath(target.Name), target.Source.Base, target.Source.Branch))
	}
	for _, target := range targets {
		owner := layout.SourceOwner(target.Name)
		if target.Source == nil || owner == target.Name {
			continue
		}
		index := indexes[owner]
		resources[index].Links = append(resources[index].Links, SourceLink{Path: layout.SourceRoot(target.Name), Target: layout.SourceRoot(owner)})
	}
	return resources
}
