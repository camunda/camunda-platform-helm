package camunda

type ZeebeApplicationYAML struct {
	Zeebe ZeebeYAML `yaml:"zeebe"`
}

type ZeebeYAML struct {
	Gateway GatewayYAML `yaml:"gateway"`
	Broker  BrokerYAML  `yaml:"broker"`
}

type BrokerYAML struct {
	Exporters ExportersYAML `yaml:"exporters"`
}

type ExportersYAML struct {
	Elasticsearch ElasticsearchYAML `yaml:"elasticsearch"`
}

type ElasticsearchYAML struct {
	ClassName string `yaml:"className"`
}

type GatewayYAML struct {
	MultiTenancy MultiTenancyYAML `yaml:"multitenancy"`
}

type MultiTenancyYAML struct {
	Enabled bool `yaml:"enabled"`
}
