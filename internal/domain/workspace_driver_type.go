package domain

import "fmt"

type WorkspaceDriverType string

const Tmux WorkspaceDriverType = "tmux"

func NewWorkspaceDriverType(value string) (WorkspaceDriverType, error) {
	driver := WorkspaceDriverType(value)
	if driver == Tmux {
		return driver, nil
	}
	return driver, fmt.Errorf("invalid workspace driver type %q", driver)
}
