package domain

type SessionTemplate struct{ Windows []Window }

func NewSessionTemplate(windows []Window) SessionTemplate {
	return SessionTemplate{Windows: append([]Window(nil), windows...)}
}
