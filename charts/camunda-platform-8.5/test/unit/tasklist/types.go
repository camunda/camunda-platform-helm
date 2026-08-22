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

package tasklist

type TasklistConfigYAML struct {
	Server          ServerYAML          `yaml:"server"`
	Spring          SpringYAML          `yaml:"spring"`
	Camunda         CamundaYAML         `yaml:"camunda"`
	Security        SecurityYAML        `yaml:"security"`
	CamundaTasklist CamundaTasklistYAML `yaml:"camunda.tasklist"`
}

type ServerYAML struct {
	Servlet ServletYAML `yaml:"servlet"`
}

type ServletYAML struct {
	ContextPath string `yaml:"contextPath"`
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
	ClientId string `yaml:"clientId"`
}

type SecurityYAML struct {
	OAuth2 OAuth2YAML `yaml:"oauth2"`
}

type OAuth2YAML struct {
	ResourceServer ResourceServerYAML `yaml:"resourceserver"`
}

type ResourceServerYAML struct {
	JWT JWTYAML `yaml:"jwt"`
}

type JWTYAML struct {
	IssuerURI string `yaml:"issuer-uri"`
}

type CamundaTasklistYAML struct {
	Identity     TasklistIdentityYAML `yaml:"identity"`
	MultiTenancy MultiTenancyYAML     `yaml:"multiTenancy"`
}
type TasklistIdentityYAML struct {
	RedirectRootURL string `yaml:"redirectRootUrl"`
}

type MultiTenancyYAML struct {
	Enabled string `yaml:"enabled"`
}
