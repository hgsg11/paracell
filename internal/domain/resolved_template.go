package domain

type ResolvedTemplate struct {
	Name         string
	Commander    *CommanderCellSpec
	Targets      []TargetCellSpec
	Dependencies []DependencyCellSpec
}

func NewResolvedTemplate(name string, commander CommanderCellSpec, targets []TargetCellSpec, dependencies []DependencyCellSpec) ResolvedTemplate {
	return ResolvedTemplate{Name: name, Commander: &commander, Targets: targets, Dependencies: dependencies}
}
