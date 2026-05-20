package domain

type ProjectConfig struct {
	Name string
}

type Config struct {
	Project   ProjectConfig
	Templates map[string]Template
}
