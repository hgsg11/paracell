package domain

import (
	"fmt"
	"path/filepath"
	"strings"
)

type SourceTemplate struct {
	Path string
	Base string
}

func NewSourceTemplate(path string, base string) (SourceTemplate, error) {
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
	return SourceTemplate{Path: clean, Base: base}, nil
}

func NewPartialSourceTemplate(path *string, base *string) (SourceTemplate, error) {
	pathValue, baseValue := "", ""
	if path != nil {
		pathValue = *path
	}
	if base != nil {
		baseValue = *base
	}
	return NewSourceTemplate(pathValue, baseValue)
}
