package domain

type SourceResource struct {
	RepositoryPath string
	WorktreePath   string
	Base           string
	Branch         string
}

func NewSourceResource(repositoryPath string, worktreePath string, base string, branch string) SourceResource {
	return SourceResource{RepositoryPath: repositoryPath, WorktreePath: worktreePath, Base: base, Branch: branch}
}
