package domain

import "fmt"

type Window struct {
	Name    string
	Command string
}

func NewWindow(name string, command string) (Window, error) {
	if name == "" {
		return Window{}, fmt.Errorf("window name is required")
	}
	return Window{Name: name, Command: command}, nil
}
