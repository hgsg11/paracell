package domain

type ResolvedTemplate struct {
	Name       string
	Sources    []SourceTemplate
	Containers []ContainerTemplate
	Session    SessionTemplate
}

func NewResolvedTemplate(name string, sources []SourceTemplate, containers []ContainerTemplate, session SessionTemplate) ResolvedTemplate {
	return ResolvedTemplate{Name: name, Sources: append([]SourceTemplate(nil), sources...), Containers: append([]ContainerTemplate(nil), containers...), Session: NewSessionTemplate(session.Windows)}
}
