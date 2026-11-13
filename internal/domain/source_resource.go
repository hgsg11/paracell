package domain

type SourceResource struct {
	RepositoryPath string
	WorktreePath   string
	Base           string
	Branch         string
	Links          []SourceLink
}

// SourceLink is a link from a TargetCell's source directory to the source
// directory of the TargetCell that owns the shared worktree. Both paths are
// relative to the project root.
type SourceLink struct {
	Path   string
	Target string
}

func NewSourceResource(repositoryPath string, worktreePath string, base string, branch string) SourceResource {
	return SourceResource{RepositoryPath: repositoryPath, WorktreePath: worktreePath, Base: base, Branch: branch}
}
