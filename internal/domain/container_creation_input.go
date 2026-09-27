package domain

type ContainerCreationInput struct {
	Containers []Container
	CellName   string
	Project    string
	Network    string
	SourcePath string
}

func NewContainerCreationInput(containers []Container, cellName string, project string, network string, sourcePath string) ContainerCreationInput {
	return ContainerCreationInput{
		Containers: append([]Container(nil), containers...),
		CellName:   cellName,
		Project:    project,
		Network:    network,
		SourcePath: sourcePath,
	}
}
