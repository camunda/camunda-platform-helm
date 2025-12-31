package chartcomponents

// ValuesYAML88 represents the structure of values.yaml for chart version 8.8
// In 8.8, zeebe/zeebeGateway/operate/tasklist are replaced by orchestration
type ValuesYAML88 struct {
	Identity      *ComponentImage `yaml:"identity,omitempty"`
	Console       *ComponentImage `yaml:"console,omitempty"`
	WebModeler    *ComponentImage `yaml:"webModeler,omitempty"`
	Connectors    *ComponentImage `yaml:"connectors,omitempty"`
	Orchestration *ComponentImage `yaml:"orchestration,omitempty"`
	Optimize      *ComponentImage `yaml:"optimize,omitempty"`
}
