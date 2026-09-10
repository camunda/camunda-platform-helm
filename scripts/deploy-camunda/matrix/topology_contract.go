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

package matrix

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type optimizeSourceLayer struct {
	Optimize struct {
		ContextPath *string `yaml:"contextPath"`
		Database    struct {
			Elasticsearch optimizeSourceBackend `yaml:"elasticsearch"`
			Opensearch    optimizeSourceBackend `yaml:"opensearch"`
		} `yaml:"database"`
	} `yaml:"optimize"`
}

type optimizeSourceBackend struct {
	Prefix *string `yaml:"prefix"`
}

func readOptimizeSourceLayer(path string) (optimizeSourceLayer, bool) {
	content, err := os.ReadFile(path)
	if err != nil {
		return optimizeSourceLayer{}, false
	}
	var layer optimizeSourceLayer
	if yaml.Unmarshal(content, &layer) != nil {
		return optimizeSourceLayer{}, false
	}
	return layer, true
}

func placeholderForms(name string) []string {
	return []string{"${" + name + "}", "$" + name}
}

func isExactPlaceholder(value, name string) bool {
	return value == "${"+name+"}" || value == "$"+name
}

func placeholderLeads(value, name string) bool {
	if isExactPlaceholder(value, name) || strings.HasPrefix(value, "${"+name+"}") {
		return true
	}
	rest, ok := strings.CutPrefix(value, "$"+name)
	return ok && rest != "" && !isShellNameByte(rest[0])
}

func isShellNameByte(b byte) bool {
	return b == '_' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

func validateOptimizeLayerSources(label string, release TopologyRelease, chartFullSetupDir string) []string {
	readAny := false
	contextSet := false
	prefixSet := false
	var problems []string
	for _, feature := range release.Features {
		if !isPlainFilename(feature) {
			continue
		}
		layer, ok := readOptimizeSourceLayer(filepath.Join(chartFullSetupDir, "values", "features", feature+".yaml"))
		if !ok {
			continue
		}
		readAny = true
		if value := layer.Optimize.ContextPath; value != nil && *value != "" {
			contextSet = true
			if !isExactPlaceholder(*value, "RELEASE_OPTIMIZE_CONTEXT_PATH") {
				problems = append(problems, fmt.Sprintf("%s: feature %q sets optimize.contextPath %q but the release declares optimize-context-path %q; set it to exactly %q", label, feature, *value, release.OptimizeContextPath, placeholderForms("RELEASE_OPTIMIZE_CONTEXT_PATH")[0]))
			}
		}
		for name, value := range map[string]*string{
			"elasticsearch": layer.Optimize.Database.Elasticsearch.Prefix,
			"opensearch":    layer.Optimize.Database.Opensearch.Prefix,
		} {
			if value == nil || *value == "" {
				continue
			}
			prefixSet = true
			if !placeholderLeads(*value, "SERVED_ORCHESTRATION_INDEX_PREFIX") {
				problems = append(problems, fmt.Sprintf("%s: feature %q sets optimize.database.%s.prefix to %q; it must start with %q so it follows serves %q", label, feature, name, *value, placeholderForms("SERVED_ORCHESTRATION_INDEX_PREFIX")[0], release.Serves))
			}
		}
	}
	if !readAny {
		return problems
	}
	if !contextSet {
		problems = append(problems, fmt.Sprintf("%s: no feature layer sets optimize.contextPath; set it to %q", label, placeholderForms("RELEASE_OPTIMIZE_CONTEXT_PATH")[0]))
	}
	if !prefixSet {
		problems = append(problems, fmt.Sprintf("%s: no feature layer sets optimize.database.elasticsearch.prefix or optimize.database.opensearch.prefix to %q", label, placeholderForms("SERVED_ORCHESTRATION_INDEX_PREFIX")[0]))
	}
	return problems
}

type TopologyContractSecret struct {
	Kind  string `json:"kind"`
	Name  string `json:"name"`
	Key   string `json:"key"`
	Token string `json:"token"`
}

type TopologyContractOptimize struct {
	Enabled     bool                   `json:"enabled"`
	ContextPath string                 `json:"contextPath"`
	Backend     string                 `json:"backend"`
	IndexPrefix string                 `json:"indexPrefix"`
	ClientID    string                 `json:"clientId"`
	Audience    string                 `json:"audience"`
	RedirectURL string                 `json:"redirectUrl"`
	Secret      TopologyContractSecret `json:"secret"`
}

type TopologyContractCluster struct {
	ID                  string                   `json:"id"`
	OptimizeContextPath string                   `json:"optimizeContextPath"`
	Optimize            TopologyContractOptimize `json:"optimize"`
}

type TopologyContractIdentityClient struct {
	ID                string                 `json:"id"`
	RootURL           string                 `json:"rootUrl"`
	RedirectURIs      []string               `json:"redirectUris"`
	ResourceServerIDs []string               `json:"resourceServerIds"`
	Secret            TopologyContractSecret `json:"secret"`
}

type TopologyContract struct {
	Optimize      TopologyContractOptimize `json:"optimize"`
	Orchestration struct {
		ElasticsearchIndexPrefix string `json:"elasticsearchIndexPrefix"`
		OpensearchIndexPrefix    string `json:"opensearchIndexPrefix"`
	} `json:"orchestration"`
	Hub struct {
		AuthType         string                           `json:"authType"`
		ClustersDeclared bool                             `json:"clustersDeclared"`
		Clusters         []TopologyContractCluster        `json:"clusters"`
		IdentityClients  []TopologyContractIdentityClient `json:"identityClients"`
	} `json:"hub"`
}

type RenderedTopologyRelease struct {
	Release  TopologyRelease
	Contract TopologyContract
}

func (t *Topology) ValidateRendered(ctx string, rendered []RenderedTopologyRelease) error {
	if t == nil {
		return nil
	}
	bySuffix := make(map[string]RenderedTopologyRelease, len(rendered))
	var hub TopologyContract
	orchestrations := make(map[string]TopologyContract, len(rendered))
	for _, release := range rendered {
		bySuffix[release.Release.NamespaceSuffix] = release
		switch release.Release.Role {
		case "hub":
			hub = release.Contract
		case "orchestration":
			orchestrations[release.Release.NamespaceSuffix] = release.Contract
		}
	}

	var problems []string
	for _, declared := range t.Releases {
		if declared.Role != "optimize" {
			continue
		}
		actual, ok := bySuffix[declared.NamespaceSuffix]
		label := fmt.Sprintf("%s: topology %q: release (role %q, namespace-suffix %q)", ctx, t.Name, declared.Role, declared.NamespaceSuffix)
		if !ok {
			problems = append(problems, label+": rendered contract is missing")
			continue
		}
		problems = append(problems, validateRenderedOptimize(label, t, declared, actual.Contract.Optimize, hub, orchestrations[declared.Serves])...)
	}
	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("%s", strings.Join(problems, "\n  - "))
}

func validateRenderedOptimize(label string, topology *Topology, release TopologyRelease, optimize TopologyContractOptimize, hub, orchestration TopologyContract) []string {
	var problems []string
	if optimize.ContextPath != release.OptimizeContextPath {
		problems = append(problems, fmt.Sprintf("%s: rendered Optimize context path is %q, want %q", label, optimize.ContextPath, release.OptimizeContextPath))
	}
	writerPrefix := orchestration.Orchestration.ElasticsearchIndexPrefix
	if optimize.Backend == "opensearch" {
		writerPrefix = orchestration.Orchestration.OpensearchIndexPrefix
	}
	if optimize.Backend != "none" && writerPrefix != "" && !strings.HasPrefix(optimize.IndexPrefix, writerPrefix) {
		problems = append(problems, fmt.Sprintf("%s: rendered Optimize %s index prefix is %q, want the served orchestration writer prefix %q", label, optimize.Backend, optimize.IndexPrefix, writerPrefix))
	}

	clusterID := ""
	for _, candidate := range topology.Releases {
		if candidate.Role == "orchestration" && candidate.NamespaceSuffix == release.Serves {
			clusterID = candidate.ModelerClusterID
		}
	}
	var cluster *TopologyContractCluster
	for i := range hub.Hub.Clusters {
		if hub.Hub.Clusters[i].ID == clusterID {
			cluster = &hub.Hub.Clusters[i]
			break
		}
	}
	if cluster == nil {
		if !hub.Hub.ClustersDeclared {
			return problems
		}
		return append(problems, validateStandaloneOptimize(label, optimize, hub, fmt.Sprintf("the Hub inventory, which has no cluster record with id %q", clusterID))...)
	}
	registered := cluster.Optimize
	location := fmt.Sprintf("global.topology.clusters[id=%q].components.optimize", clusterID)
	if registered.ClientID != optimize.ClientID {
		return append(problems, validateStandaloneOptimize(label, optimize, hub, fmt.Sprintf("the Hub registers this cluster's Optimize client id as %q (%s)", registered.ClientID, location))...)
	}
	if cluster.OptimizeContextPath != release.OptimizeContextPath {
		problems = append(problems, fmt.Sprintf("%s: the Hub advertises this cluster's Optimize at %q, want %q", label, cluster.OptimizeContextPath, release.OptimizeContextPath))
	}
	if !registered.Enabled {
		return append(problems, fmt.Sprintf("%s: %s.enabled must be true so the Hub provisions this Optimize release", label, location))
	}
	if registered.Audience != optimize.Audience {
		problems = append(problems, fmt.Sprintf("%s: Optimize audience %q differs; the Hub registers this cluster's Optimize audience as %q", label, optimize.Audience, registered.Audience))
	}
	if registered.RedirectURL != optimize.RedirectURL {
		problems = append(problems, fmt.Sprintf("%s: optimize.security.authentication.oidc.redirectUrl is %q but the Hub registers %q", label, optimize.RedirectURL, registered.RedirectURL))
	}
	if registered.Secret != optimize.Secret {
		problems = append(problems, secretMismatch(label, "Hub cluster record", registered.Secret, optimize.Secret))
	}
	return problems
}

func validateStandaloneOptimize(label string, optimize TopologyContractOptimize, hub TopologyContract, registration string) []string {
	var client *TopologyContractIdentityClient
	for i := range hub.Hub.IdentityClients {
		if hub.Hub.IdentityClients[i].ID == optimize.ClientID {
			client = &hub.Hub.IdentityClients[i]
			break
		}
	}
	if client == nil {
		return []string{fmt.Sprintf("%s: Optimize client id %q is not represented by %s and has no matching identity.clients entry, so nothing provisions this release's client", label, optimize.ClientID, registration)}
	}
	keycloak := strings.EqualFold(hub.Hub.AuthType, "KEYCLOAK")
	key := fmt.Sprintf("identity.clients[id=%q]", client.ID)
	var redirects []string
	for _, redirect := range client.RedirectURIs {
		if strings.HasPrefix(redirect, "http://") || strings.HasPrefix(redirect, "https://") {
			redirects = append(redirects, redirect)
		}
	}
	if client.RootURL != "" {
		redirects = append(redirects, client.RootURL)
	}
	var problems []string
	matched := false
	for _, redirect := range redirects {
		matched = matched || redirect == optimize.RedirectURL
	}
	if len(redirects) == 0 && keycloak {
		problems = append(problems, fmt.Sprintf("%s: %s has no rootUrl and no absolute redirectUris", label, key))
	} else if len(redirects) > 0 && !matched && (keycloak || client.RootURL != "" || len(client.RedirectURIs) > 0) {
		problems = append(problems, fmt.Sprintf("%s: Optimize redirect URL %q differs; the Hub registers this client's redirect as %q (%s.rootUrl)", label, optimize.RedirectURL, redirects, key))
	}
	permitted := false
	for _, audience := range client.ResourceServerIDs {
		permitted = permitted || audience == optimize.Audience
	}
	if !permitted && (keycloak || len(client.ResourceServerIDs) > 0) {
		if len(client.ResourceServerIDs) == 0 {
			problems = append(problems, fmt.Sprintf("%s: %s has no permission naming a resource server", label, key))
		} else {
			problems = append(problems, fmt.Sprintf("%s: the Hub permits this client only on %s; Optimize audience is %q", label, strings.Join(client.ResourceServerIDs, ", "), optimize.Audience))
		}
	}
	if client.Secret.Kind == "" && keycloak {
		problems = append(problems, fmt.Sprintf("%s: %s requires a usable secret", label, key))
	} else if client.Secret != optimize.Secret {
		problems = append(problems, secretMismatch(label, key, client.Secret, optimize.Secret))
	}
	return problems
}

func secretMismatch(label, registration string, registered, presented TopologyContractSecret) string {
	detail := "state it in different forms"
	if registered.Kind == "inline" && presented.Kind == "inline" {
		detail = "are different literals (compared as redacted hashes)"
	}
	return fmt.Sprintf("%s: Optimize and %s client secrets %s", label, registration, detail)
}
