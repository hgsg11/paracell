package domain

// BuildContainerResourcesService builds the container resources of a CellGroup.
// Templates are looked up per TargetCell, because several TargetCells may
// declare the same container with different settings.
func BuildContainerResourcesService(group CellGroup, targets []TargetCell, dependencies []DependencyCell, templates map[ContainerTemplateKey]ContainerTemplate) ContainerResources {
	layout := BuildCellResourceLayoutService(group, targets)
	items := make([]ContainerResource, 0, len(targets)+len(dependencies))
	for _, target := range targets {
		for _, container := range target.Containers {
			template := templates[ContainerTemplateKey{Target: target.Name, Container: container.SourceContainer}]
			name := layout.ContainerName(target.Name, container.SourceContainer)
			item := NewContainerResource(name, container.Network, container.SourceContainer, container.Mode, template.Environments, template.Mounts)
			item.Target = target.Name
			item.Role = layout.ContainerRole(target.Name, container.SourceContainer)
			if target.Source != nil {
				item.SourcePath = layout.SourceWorktreePath(target.Name)
			}
			items = append(items, item)
		}
	}
	for _, dependency := range dependencies {
		items = append(items, NewContainerResource(dependency.Container.SourceContainer, dependency.Container.Network, dependency.Container.SourceContainer, dependency.Container.Mode, nil, nil))
	}
	return NewContainerResources(group.Name().Value, group.Project, group.ResourcePrefix(), items)
}
