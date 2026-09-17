package domain

type Sources struct {
	Driver SourceDriverType
	Items  []Source
}

func NewSources(driver SourceDriverType, items []Source) Sources {
	return Sources{Driver: driver, Items: append([]Source(nil), items...)}
}

func buildSources(driver SourceDriverType, templates []SourceTemplate, issue string) (Sources, error) {
	items := make([]Source, 0, len(templates))
	for _, template := range templates {
		source, err := NewSource(template.Path, template.Base, template.Prefix+issue)
		if err != nil {
			return Sources{}, err
		}
		items = append(items, source)
	}
	return NewSources(driver, items), nil
}
