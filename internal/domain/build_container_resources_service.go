package domain

import "path/filepath"

func BuildContainerResourcesService(commander CommanderCell, targets []TargetCell, dependencies []DependencyCell, templates map[string]ContainerTemplate) ContainerResources {
	items := make([]ContainerResource, 0, len(targets)+len(dependencies))
	for _, target := range targets {
		if target.Container == nil {
			continue
		}
		template := templates[target.Container.SourceContainer]
		name := commander.ResourcePrefix() + "-" + SafeResourceName(target.Name, "target") + "-" + SafeResourceName(target.Container.SourceContainer, "container")
		item := NewContainerResource(name, target.Container.Network, target.Container.SourceContainer, target.Container.Mode, template.Environments, template.Mounts)
		if target.Source != nil {
			item.SourcePath = commander.SourceWorktreePath(target.Name)
			if target.Source.Path != "." {
				item.SourcePath = filepath.Join(item.SourcePath, target.Source.Path)
			}
		}
		items = append(items, item)
	}
	for _, dependency := range dependencies {
		template := templates[dependency.Container.SourceContainer]
		items = append(items, NewContainerResource(dependency.Container.SourceContainer, dependency.Container.Network, dependency.Container.SourceContainer, dependency.Container.Mode, template.Environments, template.Mounts))
	}
	return NewContainerResources(commander.Name().Value, commander.CellGroup.Project, commander.ResourcePrefix(), items)
}
