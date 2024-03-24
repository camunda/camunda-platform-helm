package identity

type IdentityConfigYAML struct {
	Identity IdentityYAML `yaml:"identity"`
	Server   ServerYAML   `yaml:"server"`
}

type IdentityYAML struct {
	Url string `yaml:"url"`
}

type ServerYAML struct {
	Servlet ServletYAML `yaml:"servlet"`
}

type ServletYAML struct {
	ContextPath string `yaml:"context-path"`
}
