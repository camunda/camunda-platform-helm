package deploy

import (
	_ "embed"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed data/test-secret-mapping.yaml
var testSecretMappingYAML []byte

// testSecretMappingDoc is the parsed shape of data/test-secret-mapping.yaml.
type testSecretMappingDoc struct {
	Path string   `yaml:"path"`
	Vars []string `yaml:"vars"`
}

// embeddedTestSecretMapping builds the vault_secret_mapping string from the
// embedded data file. All variables share a single vault path, so the mapping
// is emitted as one comma-keyed entry ("<path> VAR1,VAR2,...") rather than
// repeating the path per variable. The result is consumed by
// vault-secret-mapper, whose parser accepts comma-separated keys.
// TestSecretMapping exposes the embedded vault_secret_mapping string to callers
// outside this package (the matrix runner). It returns the same mapping
// prepareScenarioValues resolves at deploy time.
func TestSecretMapping() (string, error) {
	return embeddedTestSecretMapping()
}

func embeddedTestSecretMapping() (string, error) {
	var doc testSecretMappingDoc
	if err := yaml.Unmarshal(testSecretMappingYAML, &doc); err != nil {
		return "", fmt.Errorf("parse embedded test-secret-mapping.yaml: %w", err)
	}
	if doc.Path == "" || len(doc.Vars) == 0 {
		return "", fmt.Errorf("embedded test-secret-mapping.yaml: path and vars are required")
	}
	return fmt.Sprintf("%s %s;", doc.Path, strings.Join(doc.Vars, ",")), nil
}
