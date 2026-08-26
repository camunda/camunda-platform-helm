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

package identity

type IdentityConfigYAML struct {
	Identity IdentityYAML `yaml:"identity"`
	Server   ServerYAML   `yaml:"server"`
	Spring   SpringYAML   `yaml:"spring"`
}

type IdentityYAML struct {
	Url          string           `yaml:"url"`
	Flags        FlagsYAML        `yaml:"flags"`
	AuthProvider AuthProviderYAML `yaml:"authProvider"`
}

type AuthProviderYAML struct {
	BackendUrl string `yaml:"backend-url"`
}

type ServerYAML struct {
	Servlet ServletYAML `yaml:"servlet"`
}

type ServletYAML struct {
	ContextPath string `yaml:"context-path"`
}

type FlagsYAML struct {
	MultiTenancy string `yaml:"multi-tenancy"`
}

type SpringYAML struct {
	DataSource DataSourceYAML `yaml:"datasource"`
}

type DataSourceYAML struct {
	Url      string `yaml:"url"`
	Username string `yaml:"username"`
}
