package domain

type Session struct {
	Driver  SessionDriverType
	Windows []SessionWindow
}

func NewSession(driver SessionDriverType, windows []SessionWindow) Session {
	return Session{Driver: driver, Windows: append([]SessionWindow(nil), windows...)}
}

func buildSession(driver SessionDriverType, template SessionTemplate) (Session, error) {
	windows := make([]SessionWindow, 0, len(template.Windows))
	for _, item := range template.Windows {
		window, err := NewSessionWindow(item.Name, item.Command)
		if err != nil {
			return Session{}, err
		}
		windows = append(windows, window)
	}
	return NewSession(driver, windows), nil
}
