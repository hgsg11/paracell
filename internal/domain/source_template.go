package domain

import (
	"fmt"
	"path/filepath"
	"strings"
)

type SourceTemplate struct {
	Path      string
	Base      string
	Prefix    string
	pathSet   bool
	baseSet   bool
	prefixSet bool
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
	return SourceTemplate{Path: clean, Base: base, Prefix: prefix, pathSet: true, baseSet: true, prefixSet: true}, nil
}

func NewPartialSourceTemplate(path *string, base *string, prefix *string) (SourceTemplate, error) {
	value := SourceTemplate{}
	if path != nil {
		parsed, err := NewSourceTemplate(*path, "", "")
		if err != nil {
			return SourceTemplate{}, err
		}
		value.Path, value.pathSet = parsed.Path, true
	}
	if base != nil {
		value.Base, value.baseSet = *base, true
	}
	if prefix != nil {
		value.Prefix, value.prefixSet = *prefix, true
	}
	return value, nil
}
