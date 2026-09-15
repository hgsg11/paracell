package domain

import "fmt"

type SessionDriverType string

const Tmux SessionDriverType = "tmux"

func NewSessionDriverType(value string) (SessionDriverType, error) {
	driver := SessionDriverType(value)
	if driver == Tmux {
		return driver, nil
	}
	return driver, fmt.Errorf("invalid session driver type %q", driver)
}
