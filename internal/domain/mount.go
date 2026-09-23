package domain

import (
	"fmt"
	"path/filepath"
	"strings"
)

type Mount struct {
	TargetPath string
	SourcePath string
}

func NewMount(targetPath string, sourcePath string) (Mount, error) {
	if !filepath.IsAbs(targetPath) {
		return Mount{}, fmt.Errorf("mount target path %q must be absolute", targetPath)
	}
	if filepath.IsAbs(sourcePath) {
		return Mount{}, fmt.Errorf("mount source path %q must be relative", sourcePath)
	}
	clean := filepath.Clean(sourcePath)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return Mount{}, fmt.Errorf("mount source path %q must stay within its source root", sourcePath)
	}
	return Mount{TargetPath: targetPath, SourcePath: clean}, nil
}
