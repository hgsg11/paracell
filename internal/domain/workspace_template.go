package domain

import "fmt"

type WorkspaceTemplate struct{ Windows []Window }

func NewWorkspaceTemplate(windows []Window) WorkspaceTemplate {
	return WorkspaceTemplate{Windows: append([]Window(nil), windows...)}
}

func (s WorkspaceTemplate) render(vars TemplateVars) (WorkspaceTemplate, error) {
	windows := make([]Window, 0, len(s.Windows))
	for _, window := range s.Windows {
		command, err := vars.Render(window.Command)
		if err != nil {
			return WorkspaceTemplate{}, fmt.Errorf("render workspace window %q: %w", window.Name, err)
		}
		rendered, err := NewWindow(window.Name, command)
		if err != nil {
			return WorkspaceTemplate{}, err
		}
		windows = append(windows, rendered)
	}
	return NewWorkspaceTemplate(windows), nil
}
