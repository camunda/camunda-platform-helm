package camunda

type OrchestrationApplicationYAML struct {
	Zeebe   ZeebeYAML   `yaml:"zeebe"`
	Spring  SpringYAML  `yaml:"spring"`
	Camunda CamundaYAML `yaml:"camunda"`
}

type ZeebeYAML struct {
	Gateway GatewayYAML `yaml:"gateway"`
	Broker  BrokerYAML  `yaml:"broker"`
}

type BrokerYAML struct {
	Gateway   GatewayYAML   `yaml:"gateway"`
	Exporters ExportersYAML `yaml:"exporters"`
}

type ExportersYAML struct {
	Elasticsearch   ElasticsearchYAML  `yaml:"elasticsearch"`
	CamundaExporter ElasticsearchYAML  `yaml:"camundaexporter"`
	Custom          CustomExporterYAML `yaml:"custom"`
}

type CustomExporterYAML struct {
	ClassName string                 `yaml:"className"`
	JarPath   string                 `yaml:"jarPath"`
	Args      map[string]interface{} `yaml:"args"`
}

type ElasticsearchYAML struct {
	ClassName string `yaml:"className"`
}

type GatewayYAML struct {
	MultiTenancy MultiTenancyYAML `yaml:"multitenancy"`
	Security     SecurityYAML     `yaml:"security"`
}

type SecurityYAML struct {
	Authentication AuthenticationYAML `yaml:"authentication"`
}

type AuthenticationYAML struct {
	Mode string `yaml:"mode"`
}

type MultiTenancyYAML struct {
	Enabled bool `yaml:"enabled"`
}

type SpringYAML struct {
	Profiles ProfilesYAML `yaml:"profiles"`
}

type ProfilesYAML struct {
	Active string `yaml:"active"`
}

type CamundaYAML struct {
	Identity IdentityYAML `yaml:"identity"`
}

type IdentityYAML struct {
	Audience         string `yaml:"audience"`
	IssuerBackendUrl string `yaml:"issuerBackendUrl"`
}
