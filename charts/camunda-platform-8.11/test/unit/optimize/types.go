// Copyright 2026 Camunda Services GmbH
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
