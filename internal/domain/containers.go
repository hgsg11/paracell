package domain

type Containers struct {
	Driver ContainerDriverType
	Items  []Container
}

func NewContainers(driver ContainerDriverType, items []Container) Containers {
	return Containers{Driver: driver, Items: append([]Container(nil), items...)}
}
