package domain

import "fmt"

type Environment struct {
	Name  string
	Value string
}

func NewEnvironment(name string, value string) (Environment, error) {
	if name == "" {
		return Environment{}, fmt.Errorf("environment name is required")
	}
	return Environment{Name: name, Value: value}, nil
}
