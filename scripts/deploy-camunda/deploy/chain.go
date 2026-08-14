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
