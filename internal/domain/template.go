package domain

import (
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

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
	Network  ContainerNetwork                    `yaml:"network,omitempty" json:"network,omitempty"`
	Services map[string]ContainerServiceTemplate `yaml:"services" json:"services"`
}

const (
	ContainerNetworkIsolated ContainerNetwork = "isolated"
	ContainerNetworkShared   ContainerNetwork = "shared"
)

type ContainerNetwork string

func (n ContainerNetwork) String() string {
	return string(n)
}

func (n ContainerNetwork) Validate() error {
	switch n {
	case "", ContainerNetworkIsolated, ContainerNetworkShared:
		return nil
	default:
		return fmt.Errorf("unsupported containers.network %q", n)
	}
}

func (n *ContainerNetwork) UnmarshalYAML(value *yaml.Node) error {
	var raw string
	if err := value.Decode(&raw); err != nil {
		return err
	}
	next := ContainerNetwork(raw)
	if err := next.Validate(); err != nil {
		return err
	}
	*n = next
	return nil
}

func (n *ContainerNetwork) UnmarshalJSON(data []byte) error {
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	next := ContainerNetwork(raw)
	if err := next.Validate(); err != nil {
		return err
	}
	*n = next
	return nil
}

type ContainerServiceTemplate struct {
	SourceContainer string            `yaml:"sourceContainer" json:"sourceContainer"`
	VolumeMode      string            `yaml:"volumeMode,omitempty" json:"volumeMode,omitempty"`
	Environment     map[string]string `yaml:"environment,omitempty" json:"environment,omitempty"`
	Database        *DatabaseConfig   `yaml:"database,omitempty" json:"database,omitempty"`
}

type DatabaseConfig struct {
	System    string   `yaml:"system,omitempty" json:"system,omitempty"`
	CopyMode  string   `yaml:"copyMode,omitempty" json:"copyMode,omitempty"`
	InitFiles []string `yaml:"initFiles,omitempty" json:"initFiles,omitempty"`
}

type SessionTemplate struct {
	Windows []SessionWindowTemplate `yaml:"windows" json:"windows"`
}

type SessionWindowTemplate struct {
	Name    string `yaml:"name" json:"name"`
	Command string `yaml:"command" json:"command"`
}
