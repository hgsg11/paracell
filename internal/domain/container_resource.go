package domain

type ContainerResource struct {
	Name            string
	Network         []string
	SourceContainer string
	Mode            Mode
	Environments    []Environment
	Mounts          []Mount
}

func NewContainerResource(name string, network []string, sourceContainer string, mode Mode, environments []Environment, mounts []Mount) ContainerResource {
	return ContainerResource{
		Name: name, Network: append([]string(nil), network...), SourceContainer: sourceContainer,
		Mode: mode, Environments: append([]Environment(nil), environments...), Mounts: append([]Mount(nil), mounts...),
	}
}
