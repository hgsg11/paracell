package domain

import "fmt"

type Container struct {
	Network         []string
	SourceContainer string
	Mode            Mode
}

func NewContainer(network []string, sourceContainer string, mode Mode) (Container, error) {
	if sourceContainer == "" {
		return Container{}, fmt.Errorf("source container is required")
	}
	validatedMode, err := NewMode(string(mode))
	if err != nil {
		return Container{}, err
	}
	return Container{Network: append([]string(nil), network...), SourceContainer: sourceContainer, Mode: validatedMode}, nil
}
