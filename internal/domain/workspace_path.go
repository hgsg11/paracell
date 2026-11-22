package domain

import (
	"fmt"
	"path"
	"strings"
)

// WorkspacePath is a file path relative to the workspace, the directory that
// holds paracell.yaml. It always uses "/" and never leaves the workspace.
type WorkspacePath struct {
	Value string
}

// NewWorkspacePath normalizes value to a path relative to the workspace. It
// rejects an empty path, an absolute path, a path outside the workspace, a
// directory path ending with "/", and glob characters.
func NewWorkspacePath(value string) (WorkspacePath, error) {
	if strings.TrimSpace(value) == "" {
		return WorkspacePath{}, fmt.Errorf("workspace path is required")
	}
	slashed := strings.ReplaceAll(value, `\`, "/")
	if path.IsAbs(slashed) || (len(slashed) >= 2 && slashed[1] == ':') {
		return WorkspacePath{}, fmt.Errorf("workspace path %q must be relative to the workspace", value)
	}
	if strings.HasSuffix(slashed, "/") {
		return WorkspacePath{}, fmt.Errorf("workspace path %q must be a file, not a directory", value)
	}
	if strings.ContainsAny(slashed, "*?[]{}") {
		return WorkspacePath{}, fmt.Errorf("workspace path %q must not contain glob characters", value)
	}
	cleaned := path.Clean(slashed)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return WorkspacePath{}, fmt.Errorf("workspace path %q must be inside the workspace", value)
	}
	return WorkspacePath{Value: cleaned}, nil
}

// ExistingPaths is the set of workspace paths that were found on disk.
type ExistingPaths struct {
	paths map[WorkspacePath]struct{}
}

func NewExistingPaths(paths []WorkspacePath) ExistingPaths {
	set := make(map[WorkspacePath]struct{}, len(paths))
	for _, p := range paths {
		set[p] = struct{}{}
	}
	return ExistingPaths{paths: set}
}

func (e ExistingPaths) Contains(p WorkspacePath) bool {
	_, ok := e.paths[p]
	return ok
}
