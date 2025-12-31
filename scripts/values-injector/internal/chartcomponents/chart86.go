package chartcomponents

// ValuesYAML86 represents the structure of values.yaml for chart version 8.6
// Contains all components that have image.tag fields
type ValuesYAML86 struct {
	Console      *ComponentImage `yaml:"console,omitempty"`
	Zeebe        *ComponentImage `yaml:"zeebe,omitempty"`
	ZeebeGateway *ComponentImage `yaml:"zeebeGateway,omitempty"`
	Operate      *ComponentImage `yaml:"operate,omitempty"`
	Tasklist     *ComponentImage `yaml:"tasklist,omitempty"`
	Optimize     *ComponentImage `yaml:"optimize,omitempty"`
	Identity     *ComponentImage `yaml:"identity,omitempty"`
	WebModeler   *ComponentImage `yaml:"webModeler,omitempty"`
	Connectors   *ComponentImage `yaml:"connectors,omitempty"`
}
