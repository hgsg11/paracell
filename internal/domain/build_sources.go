package domain

func BuildSources(driver SourceDriverType, templates []SourceTemplate, issue string) (Sources, error) {
	items := make([]Source, 0, len(templates))
	for _, template := range templates {
		source, err := template.Source(issue)
		if err != nil {
			return Sources{}, err
		}
		items = append(items, source)
	}
	return NewSources(driver, items), nil
}
