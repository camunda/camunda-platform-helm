package chartcomponents

// ImageTag represents the tag field within an image configuration
type ImageTag struct {
	Tag string `yaml:"tag,omitempty"`
}

// ComponentImage represents a component's image configuration
type ComponentImage struct {
	Image ImageTag `yaml:"image,omitempty"`
}
