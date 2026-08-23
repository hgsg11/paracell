package domain

type Template struct {
	Name       string
	Repository RepositoryTemplate
	Files      []string
	Containers ContainerTemplate
	Session    SessionTemplate
}

type TemplateVars struct {
	Issue   string
	Name    string
	Project string
	Command string
}

type RepositoryTemplate struct {
	BranchPrefix string `yaml:"branchPrefix" json:"branchPrefix"`
	Base         string `yaml:"base" json:"base"`
	BranchMode   string `yaml:"branchMode,omitempty" json:"branchMode,omitempty"`
}

const (
	RepositoryBranchModeCreate  = "create"
	RepositoryBranchModeReuse   = "reuse"
	RepositoryBranchModeRequire = "require"
)

type ContainerTemplate struct {
	Services map[string]ContainerServiceTemplate `yaml:"services" json:"services"`
}

type ContainerServiceTemplate struct {
	SourceContainer string            `yaml:"sourceContainer" json:"sourceContainer"`
	VolumeMode      string            `yaml:"volumeMode,omitempty" json:"volumeMode,omitempty"`
	Environment     map[string]string `yaml:"environment,omitempty" json:"environment,omitempty"`
	Database        *DatabaseConfig   `yaml:"database,omitempty" json:"database,omitempty"`
}

type DatabaseConfig struct {
	Mode      string   `yaml:"mode,omitempty" json:"mode,omitempty"`
	System    string   `yaml:"system,omitempty" json:"system,omitempty"`
	CopyMode  string   `yaml:"copyMode,omitempty" json:"copyMode,omitempty"`
	InitFiles []string `yaml:"initFiles,omitempty" json:"initFiles,omitempty"`
}

const (
	DatabaseModeCopy   = "copy"
	DatabaseModeShared = "shared"
)

type SessionTemplate struct {
	Windows []SessionWindowTemplate `yaml:"windows" json:"windows"`
}

type SessionWindowTemplate struct {
	Name    string `yaml:"name" json:"name"`
	Command string `yaml:"command" json:"command"`
}
