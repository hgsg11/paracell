package domain

// ContainerResources are the containers of one CellGroup. CellDirectory is the
// Cell directory relative to the project root, or empty when the CellGroup has
// no source-backed TargetCell and so no Cell directory. Paths inside the
// project root resolve to the same relative path inside it.
type ContainerResources struct {
	CellName      string
	Project       string
	Network       string
	CellDirectory string
	Items         []ContainerResource
}

func NewContainerResources(cellName string, project string, network string, cellDirectory string, items []ContainerResource) ContainerResources {
	return ContainerResources{CellName: cellName, Project: project, Network: network, CellDirectory: cellDirectory, Items: append([]ContainerResource(nil), items...)}
}
