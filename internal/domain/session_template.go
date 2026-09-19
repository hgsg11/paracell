package domain

import "fmt"

type SessionTemplate struct{ Windows []Window }

func NewSessionTemplate(windows []Window) SessionTemplate {
	return SessionTemplate{Windows: append([]Window(nil), windows...)}
}

func (s SessionTemplate) render(vars TemplateVars) (SessionTemplate, error) {
	windows := make([]Window, 0, len(s.Windows))
	for _, window := range s.Windows {
		command, err := vars.Render(window.Command)
		if err != nil {
			return SessionTemplate{}, fmt.Errorf("render session window %q: %w", window.Name, err)
		}
		rendered, err := NewWindow(window.Name, command)
		if err != nil {
			return SessionTemplate{}, err
		}
		windows = append(windows, rendered)
	}
	return NewSessionTemplate(windows), nil
}
