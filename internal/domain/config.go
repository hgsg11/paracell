package domain

type ProjectConfig struct {
	Name string
}

type ProviderConfig struct {
	Source        string
	Container     string
	Session       string
	Notifications string
}

type Config struct {
	Project   ProjectConfig
	Providers ProviderConfig
	Templates map[string]Template
}
