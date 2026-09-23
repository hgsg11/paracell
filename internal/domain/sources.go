package domain

type Sources struct {
	Driver SourceDriverType
	Items  []Source
}

func NewSources(driver SourceDriverType, items []Source) Sources {
	return Sources{Driver: driver, Items: append([]Source(nil), items...)}
}
