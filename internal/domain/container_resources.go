package domain

type ContainerResources struct {
	CellName string
	Project  string
	Network  string
	Items    []ContainerResource
}

func NewContainerResources(cellName string, project string, network string, items []ContainerResource) ContainerResources {
	return ContainerResources{CellName: cellName, Project: project, Network: network, Items: append([]ContainerResource(nil), items...)}
}
