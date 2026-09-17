package domain

type Containers struct {
	Driver ContainerDriverType
	Items  []Container
}

func NewContainers(driver ContainerDriverType, items []Container) Containers {
	return Containers{Driver: driver, Items: append([]Container(nil), items...)}
}

func buildContainers(driver ContainerDriverType, templates []ContainerTemplate) (Containers, error) {
	items := make([]Container, 0, len(templates))
	for _, template := range templates {
		container, err := NewContainer(nil, template.Name, template.Mode)
		if err != nil {
			return Containers{}, err
		}
		items = append(items, container)
	}
	return NewContainers(driver, items), nil
}
