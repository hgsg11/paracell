package domain

import (
	"fmt"
)

type Source struct {
	Path     string
	Worktree string
	Base     string
	Branch   string
}

func NewSource(path string, worktree string, base string, branch string) (Source, error) {
	template, err := NewSourceTemplate(path, base, "")
	if err != nil {
		return Source{}, err
	}
	if branch == "" {
		return Source{}, fmt.Errorf("source branch is required")
	}
	return Source{Path: template.Path, Worktree: worktree, Base: base, Branch: branch}, nil
}
