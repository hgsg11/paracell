package domain

import "fmt"

// Templates is unresolved project configuration. Inheritance and runtime
// rendering intentionally happen in ResolveTemplate, not in the config adapter.
type Templates struct {
	ProjectName            string
	Templates              []Template
	SessionDriverType      SessionDriverType
	ContainerDriverType    ContainerDriverType
	SourceDriverType       SourceDriverType
	NotificationDriverType NotificationDriverType
}

func NewTemplates(projectName string, templates []Template, sessionDriverType SessionDriverType, containerDriverType ContainerDriverType, sourceDriverType SourceDriverType, notificationDriverType NotificationDriverType) (Templates, error) {
	names := make(map[string]struct{}, len(templates))
	for _, item := range templates {
		if item.Name == "" {
			return Templates{}, fmt.Errorf("template name is required")
		}
		if _, exists := names[item.Name]; exists {
			return Templates{}, fmt.Errorf("duplicate template %q", item.Name)
		}
		names[item.Name] = struct{}{}
	}
	return Templates{
		ProjectName: projectName, Templates: append([]Template(nil), templates...),
		SessionDriverType: sessionDriverType, ContainerDriverType: containerDriverType,
		SourceDriverType: sourceDriverType, NotificationDriverType: notificationDriverType,
	}, nil
}
