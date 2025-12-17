package optimize

type OptimizeConfigYAML struct {
	Container ContainerYAML `yaml:"container"`
	Zeebe     ZeebeYAML     `yaml:"zeebe"`
	Multitenancy MultitenancyYAML `yaml:"multitenancy"`
}

type ContainerYAML struct {
	ContextPath string `yaml:"contextPath"`
}

type ZeebeYAML struct {
	Name string `yaml:"name"`
}


type MultitenancyYAML struct {
	Enabled bool `yaml:"enabled"`
}
