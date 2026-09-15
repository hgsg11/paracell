package domain

import "fmt"

type Mode string

const (
	Target     Mode = "target"
	Dependency Mode = "dependency"
)

func NewMode(value string) (Mode, error) {
	mode := Mode(value)
	switch mode {
	case Target, Dependency:
		return mode, nil
	default:
		return mode, fmt.Errorf("invalid mode %q", mode)
	}
}
