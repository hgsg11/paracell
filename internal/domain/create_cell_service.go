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
	containers, err := BuildContainers(containerDriver, resolved.Containers)
	if err != nil {
		return Cell{}, err
	}
	session, err := BuildSession(sessionDriver, resolved.Session)
	if err != nil {
		return Cell{}, err
	}
	cell, err := NewCell(id, issue, project, templateName, sources, containers, session, notificationDriver)
	if err != nil {
		return Cell{}, err
	}
	if note != nil {
		if err := cell.SetNote(*note); err != nil {
			return Cell{}, err
		}
	}
	return cell, nil
}
