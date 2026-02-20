package optimize

type OptimizeConfigYAML struct {
	Container ContainerYAML `yaml:"container"`
	Zeebe     ZeebeYAML     `yaml:"zeebe"`
	Es        EsYAML        `yaml:"es"`
}

type ContainerYAML struct {
	ContextPath string `yaml:"contextPath"`
}

type ZeebeYAML struct {
	Name string `yaml:"name"`
}

type EsYAML struct {
	Connection EsConnectionYAML `yaml:"connection"`
	Security   EsSecurityYAML   `yaml:"security"`
}

type EsConnectionYAML struct {
	Nodes []EsNodeYAML `yaml:"nodes"`
}

type EsNodeYAML struct {
	Host     string `yaml:"host"`
	HttpPort int    `yaml:"httpPort"`
}

type EsSecurityYAML struct {
	Username string    `yaml:"username"`
	Ssl      EsSslYAML `yaml:"ssl"`
}

type EsSslYAML struct {
	Enabled string `yaml:"enabled"`
}
