package domain

import "path/filepath"

func BuildContainerResourcesService(group CellGroup, targets []TargetCell, dependencies []DependencyCell, templates map[string]ContainerTemplate) ContainerResources {
	items := make([]ContainerResource, 0, len(targets)+len(dependencies))
	for _, target := range targets {
		for _, container := range target.Containers {
			template := templates[container.SourceContainer]
			name := group.ResourcePrefix() + "-" + SafeResourceName(target.Name, "target") + "-" + SafeResourceName(container.SourceContainer, "container")
			item := NewContainerResource(name, container.Network, container.SourceContainer, container.Mode, template.Environments, template.Mounts)
			if target.Source != nil {
				item.SourcePath = group.SourceWorktreePath(target.Name)
				if target.Source.Path != "." {
					item.SourcePath = filepath.Join(item.SourcePath, target.Source.Path)
				}
			}
			items = append(items, item)
		}
	}
	for _, dependency := range dependencies {
		template := templates[dependency.Container.SourceContainer]
		items = append(items, NewContainerResource(dependency.Container.SourceContainer, dependency.Container.Network, dependency.Container.SourceContainer, dependency.Container.Mode, template.Environments, template.Mounts))
	}
	return NewContainerResources(group.Name().Value, group.Project, group.ResourcePrefix(), items)
}
