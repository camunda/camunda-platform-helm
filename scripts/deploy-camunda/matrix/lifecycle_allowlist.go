// Copyright 2025 Camunda Services GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package matrix

// preSetupScriptAllowlist names files inside pre-setup-scripts/ that are
// permitted to exist without being referenced by any LifecycleHook. These
// files exist for purposes other than runner-driven hook execution and must
// be hand-audited when added. Consumed by RegistryValidator's load-time
// orphan walk and by TestLifecycleFixtures's cross-version dead-entry check.
//
//	pre-install-upgrade.sh         — sed-target marker for values-file
//	                                 uncommenting (alpha8 backwards-compat),
//	                                 not invoked by the matrix runner.
//	create-opensearch-tls-secrets.sh — helper sourced by
//	                                 pre-install-opensearch-self-signed*.sh,
//	                                 never invoked by the runner directly.
//	create-elasticsearch-tls-secrets.sh — helper sourced by
//	                                 pre-install-elasticsearch-self-signed*.sh
//	                                 (8.7-8.9 only; removed from 8.10.)
var preSetupScriptAllowlist = map[string]bool{
	"pre-install-upgrade.sh":              true,
	"create-elasticsearch-tls-secrets.sh": true,
	"create-opensearch-tls-secrets.sh":    true,
}

// commonResourcesAllowlist names files inside common/resources/ that are
// permitted to exist without being referenced by any LifecycleHook. These
// are fixtures kept for scenarios that are currently disabled but staged
// for activation; deleting them would force a separate PR to re-add them
// when the scenario is enabled.
//
//	postgres-createdb-job.yaml    — fixture for the disabled rdbms-external
//	                                scenario in 8.9/8.10. Pending its own enable PR.
//	postgresql-cluster.yaml       — CloudNativePG `Cluster` fixture for the
//	                                legacy `cnpg` dependency-profile (8.10).
//	                                The 8.10 registry no longer references the
//	                                profile (all scenarios moved to the
//	                                `postgresql` internal companion chart), but
//	                                the fixture is retained for opt-in use.
var commonResourcesAllowlist = map[string]bool{
	"postgres-createdb-job.yaml":  true,
	"postgresql-cluster.yaml":     true,
	"gateway-proxy-settings.yaml": true,
}
