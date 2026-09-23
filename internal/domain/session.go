package domain

type Session struct {
	Driver  SessionDriverType
	Windows []SessionWindow
}

func NewSession(driver SessionDriverType, windows []SessionWindow) Session {
	return Session{Driver: driver, Windows: append([]SessionWindow(nil), windows...)}
}
