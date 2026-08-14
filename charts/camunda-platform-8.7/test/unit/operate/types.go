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

package operate

type OperateConfigYAML struct {
	Server         ServerYAML         `yaml:"server"`
	Spring         SpringYAML         `yaml:"spring"`
	CamundaOperate CamundaOperateYAML `yaml:"camunda.operate"`
}

type ServerYAML struct {
	Servlet ServletYAML `yaml:"servlet"`
}

type ServletYAML struct {
	ContextPath string `yaml:"context-path"`
}
type SpringYAML struct {
	Profiles ProfilesYAML `yaml:"profiles"`
}

type ProfilesYAML struct {
	Active string `yaml:"active"`
}

type CamundaOperateYAML struct {
	MultiTenancy MultiTenancyYAML `yaml:"multiTenancy"`
	Identity     IdentityYAML     `yaml:"identity"`
}

type IdentityYAML struct {
	RedirectRootUrl string `yaml:"redirectRootUrl"`
}

type MultiTenancyYAML struct {
	Enabled string `yaml:"enabled"`
}
