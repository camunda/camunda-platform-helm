// Copyright Camunda Services GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	Elasticsearch   ElasticsearchYAML   `yaml:"elasticsearch"`
	CamundaExporter CamundaExporterYAML `yaml:"camundaexporter"`
}

type ElasticsearchYAML struct {
	ClassName string `yaml:"className"`
}

type CamundaExporterYAML struct {
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
	Data     DataYAML     `yaml:"data"`
}

type DataYAML struct {
	SecondaryStorage SecondaryStorageYAML `yaml:"secondary-storage"`
}

type SecondaryStorageYAML struct {
	AutoconfigureCamundaExporter bool                       `yaml:"autoconfigure-camunda-exporter"`
	Elasticsearch                DocumentSecondaryStoreYAML `yaml:"elasticsearch"`
	OpenSearch                   DocumentSecondaryStoreYAML `yaml:"opensearch"`
}

type DocumentSecondaryStoreYAML struct {
	History HistoryYAML `yaml:"history"`
}

type HistoryYAML struct {
	ElsRolloverDateFormat     string `yaml:"els-rollover-date-format"`
	RolloverInterval          string `yaml:"rollover-interval"`
	RolloverBatchSize         int    `yaml:"rollover-batch-size"`
	WaitPeriodBeforeArchiving string `yaml:"wait-period-before-archiving"`
	DelayBetweenRuns          int    `yaml:"delay-between-runs"`
	MaximumDelayBetweenRuns   int    `yaml:"max-delay-between-runs"`
}

type IdentityYAML struct {
	Audience         string `yaml:"audience"`
	IssuerBackendUrl string `yaml:"issuerBackendUrl"`
}
