package domain

func CreateCellService(id string, issue string, project string, templateName string, resolved ResolvedTemplate, sourceDriver SourceDriverType, containerDriver ContainerDriverType, sessionDriver SessionDriverType, notificationDriver NotificationDriverType, note *string) (Cell, error) {
	sourceItems := make([]Source, 0, len(resolved.Sources))
	for _, template := range resolved.Sources {
		source, err := NewSource(template.Path, template.WorktreePath(issue), template.Base, template.BranchName(issue))
		if err != nil {
			return Cell{}, err
		}
		sourceItems = append(sourceItems, source)
	}
	sources := NewSources(sourceDriver, sourceItems)

	containerItems := make([]Container, 0, len(resolved.Containers))
	for _, template := range resolved.Containers {
		container, err := NewContainer(nil, template.Name, template.Mode)
		if err != nil {
			return Cell{}, err
		}
		containerItems = append(containerItems, container)
	}
	containers := NewContainers(containerDriver, containerItems)

	windows := make([]SessionWindow, 0, len(resolved.Session.Windows))
	for _, template := range resolved.Session.Windows {
		window, err := NewSessionWindow(template.Name, template.Command)
		if err != nil {
			return Cell{}, err
		}
		windows = append(windows, window)
	}
	session := NewSession(sessionDriver, windows)

	return NewCell(id, issue, project, templateName, sources, containers, session, notificationDriver, note)
}
