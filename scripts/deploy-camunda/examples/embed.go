// Package examples exposes the deploy-camunda starter configs shipped under
// scripts/deploy-camunda/examples/ as embedded bytes so `deploy-camunda config
// init --from-example <name>` works without a checked-out repo.
package examples

import (
	"embed"
	"fmt"
	"sort"
	"strings"
)

const suffix = ".deploy-camunda.yaml"

//go:embed *.deploy-camunda.yaml
var files embed.FS

// Load returns the raw bytes of the named example. The name is the base file
// name with the `.deploy-camunda.yaml` suffix stripped, e.g. "getting-started".
func Load(name string) ([]byte, error) {
	b, err := files.ReadFile(name + suffix)
	if err != nil {
		return nil, fmt.Errorf("example %q not found; run `deploy-camunda config init --list-examples` to see available templates", name)
	}
	return b, nil
}

// Names returns the available example names (suffix stripped, sorted).
func Names() []string {
	entries, err := files.ReadDir(".")
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), suffix) {
			continue
		}
		names = append(names, strings.TrimSuffix(e.Name(), suffix))
	}
	sort.Strings(names)
	return names
}
