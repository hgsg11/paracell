package domain

func BuildSource(template SourceTemplate, issue string) (Source, error) {
	return NewSource(template.Path, template.Base, template.Prefix+issue)
}
