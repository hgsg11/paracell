package domain

import "path/filepath"

func BuildSourceResourcesService(c CommanderCell, targets []TargetCell) []SourceResource {
	resources := make([]SourceResource, 0, len(targets))
	for _, target := range targets {
		if target.Source == nil {
			continue
		}
		path := c.SourceWorktreePath(target.Name)
		if target.Source.Path != "." {
			path = filepath.Join(path, target.Source.Path)
		}
		resources = append(resources, NewSourceResource(target.Source.Path, path, target.Source.Base, target.Source.Branch))
	}
	return resources
}
