package console

type ConsoleYAML struct {
	OAuth OAuth2Config `yaml:"oAuth"`
}

type OAuth2Config struct {
	ClientId string `yaml:"clientId"`
	Type     string `yaml:"type"`
	Audience string `yaml:"audience"`
	JwksUri  string `yaml:"jwksUri"`
}
