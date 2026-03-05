package deploy

// BuildValuesChain assembles the final ordered list of Helm values files.
// This is the single canonical definition of values precedence for all code paths
// (layered, legacy, prepare-values CLI).
//
// Precedence (last wins in Helm's -f merge):
//  1. common    — shared base values (platform-specific, env-processed)
//  2. overlays  — chart-root overlays (values-latest.yaml, values-enterprise.yaml, values-digest.yaml)
//  3. extra     — user-provided --extra-values
//  4. scenario  — scenario-specific layers (identity, persistence, platform, features)
//  5. debug     — debug values file (highest precedence)
func BuildValuesChain(common, overlays, extra, scenario []string, debugFile string) []string {
	total := len(common) + len(overlays) + len(extra) + len(scenario)
	if debugFile != "" {
		total++
	}
	chain := make([]string, 0, total)
	chain = append(chain, common...)
	chain = append(chain, overlays...)
	chain = append(chain, extra...)
	chain = append(chain, scenario...)
	if debugFile != "" {
		chain = append(chain, debugFile)
	}
	return chain
}
