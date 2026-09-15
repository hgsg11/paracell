package domain

import "fmt"

type SessionWindow struct {
	Name    string
	Command string
}

func NewSessionWindow(name string, command string) (SessionWindow, error) {
	if name == "" {
		return SessionWindow{}, fmt.Errorf("session window name is required")
	}
	return SessionWindow{Name: name, Command: command}, nil
}
