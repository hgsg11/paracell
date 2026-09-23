package domain

import "fmt"

type SourceDriverType string

const Git SourceDriverType = "git"

func NewSourceDriverType(value string) (SourceDriverType, error) {
	driver := SourceDriverType(value)
	if driver == Git {
		return driver, nil
	}
	return driver, fmt.Errorf("invalid source driver type %q", driver)
}
