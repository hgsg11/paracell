package domain

type ContainerDriverType string

const (
	None   ContainerDriverType = "none"
	Docker ContainerDriverType = "docker"
)

func NewContainerDriverType(value string) ContainerDriverType {
	driver := ContainerDriverType(value)
	if driver == Docker {
		return driver
	}
	return None
}
