package domain

type ResolvedTemplate struct {
	Name      string
	Commander *CommanderCellSpec
}

func NewResolvedTemplate(name string, commander CommanderCellSpec) ResolvedTemplate {
	return ResolvedTemplate{Name: name, Commander: &commander}
}
