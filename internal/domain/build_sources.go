package domain

func BuildSource(template SourceTemplate, issue string, branchPrefix string) (Source, error) {
	return NewSource(template.Path, template.Base, branchPrefix+issue)
}
