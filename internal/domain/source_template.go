package domain

import (
	"fmt"
	"path/filepath"
	"strings"
)

type SourceTemplate struct {
	Path   string
	Base   string
	Prefix string
}

func NewSourceTemplate(path string, base string, prefix string) (SourceTemplate, error) {
	if path == "" {
		path = "."
	}
	if filepath.IsAbs(path) {
		return SourceTemplate{}, fmt.Errorf("source path %q must be relative", path)
	}
	clean := filepath.Clean(path)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return SourceTemplate{}, fmt.Errorf("source path %q must stay within project root", path)
	}
	return SourceTemplate{Path: clean, Base: base, Prefix: prefix}, nil
}

func NewPartialSourceTemplate(path *string, base *string, prefix *string) (SourceTemplate, error) {
	pathValue, baseValue, prefixValue := "", "", ""
	if path != nil {
		pathValue = *path
	}
	if base != nil {
		baseValue = *base
	}
	if prefix != nil {
		prefixValue = *prefix
	}
	return NewSourceTemplate(pathValue, baseValue, prefixValue)
}
