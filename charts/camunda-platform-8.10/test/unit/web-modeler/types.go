package web_modeler

type WebModelerRestAPIApplicationYAML struct {
	Camunda    CamundaYAML    `yaml:"camunda"`
	Spring     SpringYAML     `yaml:"spring"`
	Server     ServerYAML     `yaml:"server"`
	Management ManagementYAML `yaml:"management"`
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
	OAuth2 SpringSecurityOAuth2YAML `yaml:"oauth2"`
}

type SpringSecurityOAuth2YAML struct {
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
	Server   ModelerServerYAML   `yaml:"server"`
	OAuth2   ModelerOAuth2YAML   `yaml:"oauth2"`
	Pusher   PusherYAML          `yaml:"pusher"`
}

type ModelerSecurityYAML struct {
	JWT ModelerJwtYAML `yaml:"jwt"`
}

type ModelerServerYAML struct {
	HttpsOnly string `yaml:"https-only"`
	Url       string `yaml:"url"`
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

type PusherYAML struct {
	Client PusherClientYAML `yaml:"client"`
}

type PusherClientYAML struct {
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	Path     string `yaml:"path"`
	ForceTLS bool   `yaml:"force-tls"`
}

type ModelerOAuth2YAML struct {
	Token    TokenYAML `yaml:"token"`
	ClientId string    `yaml:"client-id"`
}

type TokenYAML struct {
	UsernameClaim string `yaml:"username-claim"`
}

type ServerYAML struct {
	Servlet ServletYAML `yaml:"servlet"`
}

type ServletYAML struct {
	ContextPath string `yaml:"context-path"`
}

type ManagementYAML struct {
	Server ManagementServerYAML `yaml:"server"`
}

type ManagementServerYAML struct {
	BasePath string `yaml:"base-path"`
}
