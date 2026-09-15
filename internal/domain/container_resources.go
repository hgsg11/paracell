package domain

type ContainerResources struct {
	CellName   string
	Project    string
	Network    string
	SourcePath string
	Items      []ContainerResource
}

func NewContainerResources(cellName string, project string, network string, sourcePath string, items []ContainerResource) ContainerResources {
	return ContainerResources{CellName: cellName, Project: project, Network: network, SourcePath: sourcePath, Items: append([]ContainerResource(nil), items...)}
}
