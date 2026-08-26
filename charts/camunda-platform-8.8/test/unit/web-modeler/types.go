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

package web_modeler

// REST API ---
type WebModelerRestAPIApplicationYAML struct {
	Camunda CamundaYAML `yaml:"camunda"`
	Spring  SpringYAML  `yaml:"spring"`
}

type SpringYAML struct {
	Mail       MailYAML           `yaml:"mail"`
	Datasource DatasourceYAML     `yaml:"datasource"`
	Security   SpringSecurityYAML `yaml:"security"`
}
type DatasourceYAML struct {
	Url      string `yaml:"url"`
	Username string `yaml:"username"`
}

type MailYAML struct {
	Username string `yaml:"username"`
}

type SpringSecurityYAML struct {
	OAuth2 OAuth2YAML `yaml:"oauth2"`
}

type OAuth2YAML struct {
	ResourceServer ResourceServerYAML `yaml:"resourceserver"`
}

type ResourceServerYAML struct {
	JWT SpringJwtYAML `yaml:"jwt"`
}

type SpringJwtYAML struct {
	JwkSetURI string `yaml:"jwk-set-uri"`
}

type CamundaYAML struct {
	Modeler  ModelerYAML  `yaml:"modeler"`
	Identity IdentityYAML `yaml:"identity"`
}

type IdentityYAML struct {
	BaseURL string `yaml:"base-url"`
	Type    string `yaml:"type"`
}
type ModelerYAML struct {
	Security ModelerSecurityYAML `yaml:"security"`
	Clusters []ClusterYAML       `yaml:"clusters"`
}

type ModelerSecurityYAML struct {
	JWT ModelerJwtYAML `yaml:"jwt"`
}

type ModelerJwtYAML struct {
	Audience AudienceYAML `yaml:"audience"`
	Issuer   IssuerYAML   `yaml:"issuer"`
}

type IssuerYAML struct {
	BackendUrl string `yaml:"backend-url"`
}

type AudienceYAML struct {
	InternalAPI string `yaml:"internal-api"`
	PublicAPI   string `yaml:"public-api"`
}

type ClusterYAML struct {
	Id             string             `yaml:"id"`
	Name           string             `yaml:"name"`
	Version        string             `yaml:"version"`
	Authentication string             `yaml:"authentication"`
	Url            UrlYAML            `yaml:"url"`
	Authorizations AuthorizationsYAML `yaml:"authorizations"`
}

type UrlYAML struct {
	Zeebe    ZeebeUrlYAML `yaml:"zeebe"`
	Operate  string       `yaml:"operate"`
	Tasklist string       `yaml:"tasklist"`
	Grpc     string       `yaml:"grpc"`
	Rest     string       `yaml:"rest"`
	WebApp   string       `yaml:"web-app"`
}

type ZeebeUrlYAML struct {
	Grpc string `yaml:"grpc"`
	Rest string `yaml:"rest"`
}

type AuthorizationsYAML struct {
	Enabled bool `yaml:"enabled"`
}

// Web App ---

type WebModelerWebAppTOML struct {
	OAuth2   OAuth2Config   `toml:"oAuth2"`
	Client   ClientConfig   `toml:"client"`
	Identity IdentityConfig `toml:"identity"`
	Server   ServerConfig   `toml:"server"`
}
type ServerConfig struct {
	HttpsOnly string `toml:"httpsOnly"`
}
type IdentityConfig struct {
	BaseUrl string `toml:"baseUrl"`
}

type ClientConfig struct {
	Pusher PusherConfig `toml:"pusher"`
}

type PusherConfig struct {
	Host     string `toml:"host"`
	Port     string `toml:"port"`
	Path     string `toml:"path"`
	ForceTLS bool   `toml:"forceTLS"`
}

type OAuth2Config struct {
	Token    TokenConfig `toml:"token"`
	ClientId string      `toml:"clientId"`
	Type     string      `toml:"type"`
}

type TokenConfig struct {
	Audience      string `toml:"audience"`
	JwksUrl       string `toml:"jwksUrl"`
	UsernameClaim string `toml:"usernameClaim"`
}
