package optimize

type OptimizeConfigYAML struct {
	Container ContainerYAML `yaml:"container"`
}

type ContainerYAML struct {
	ContextPath string `yaml:"contextPath"`
}
