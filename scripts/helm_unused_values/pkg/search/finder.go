// Package search provides functionality to search through Helm templates
// and analyze value usage
package search

import (
	"os/exec"

	"camunda.com/helm-unused-values/pkg/patterns"
)

// KeyUsage represents the analysis result for a key
type KeyUsage struct {
	Key         string
	IsUsed      bool
	UsageType   string // "direct", "pattern", "parent", "unused"
	Locations   []string
	ParentKey   string
	PatternName string
	ChildKeys   []string
}

// Finder handles searching for pattern matches and finding unused keys
type Finder struct {
	UseRipgrep   bool
	Debug        bool
	TemplatesDir string
	Registry     *patterns.Registry
	Parallelism  int  // Number of parallel workers (0 = auto)
}

// NewFinder creates a new finder
func NewFinder(templatesDir string, registry *patterns.Registry, useRipgrep, debug bool) *Finder {
	return &Finder{
		TemplatesDir: templatesDir,
		Registry:     registry,
		UseRipgrep:   useRipgrep,
		Debug:        debug,
		Parallelism:  0, // Auto
	}
}

// FindUnusedKeys analyzes the keys usage and returns the result
func (f *Finder) FindUnusedKeys(keys []string, showProgress bool) ([]KeyUsage, error) {
	// Implementation in analyzer.go
	return f.analyzeKeys(keys, showProgress)
}

// CheckDependencies checks for required external tools
func CheckDependencies() (bool, []string) {
	var missing []string

	if _, err := exec.LookPath("yq"); err != nil {
		missing = append(missing, "yq")
	}

	if _, err := exec.LookPath("jq"); err != nil {
		missing = append(missing, "jq")
	}

	return len(missing) == 0, missing
}

// DetectRipgrep checks if ripgrep is available
func DetectRipgrep() bool {
	_, err := exec.LookPath("rg")
	return err == nil
}
