---
name: camunda-helm-values
description: 'Generate Camunda Platform Helm values.yaml from natural language. Uses integration scenarios as baselines with targeted modifications.'
---

# Camunda Helm Values

## Triggers

Activate when generating `values.yaml` for Camunda Platform Helm chart.

## Prerequisites

- `helm` 3.10+ (for validation)
- `prepare-helm-values` built: `make install.prepare-helm-values`

---

## Workflow

### 1. Select Scenario + Explain

**ALWAYS output:**
```
📋 **Baseline**: `{scenario}` scenario
💡 **Why**: {best effort: what user intent matched this scenario}

🔧 **Modifications**:
- {change from baseline}
```

### 2. Generate Baseline

Use latest chart version (currently 13.4.1) unless user specifies otherwise:

**Chart → Folder mapping:**

| Chart Version | Folder | Camunda Version |
|---------------|--------|-----------------|
| **13.x.x** | `camunda-platform-8.8` | 8.8 (current) |
| 14.x.x | `camunda-platform-8.9` | 8.9 (alpha) |
| 12.x.x | `camunda-platform-8.7` | 8.7 |
| 11.x.x | `camunda-platform-8.6` | 8.6 |
| 10.x.x | `camunda-platform-8.5` | 8.5 (extended) |

```bash
CAMUNDA_HOSTNAME="{hostname}" prepare-helm-values \
  --chart-path ./charts/camunda-platform-{version} \
  --scenario {scenario} \
  --output-dir ./generated-values \
  --interactive=false
```

### 3. Apply Modifications + Validate

**Use file edit tools** (not terminal heredoc/sed) to modify `./generated-values/values-integration-test-ingress-{scenario}.yaml`.

Validate (**must pass**) using official Helm repo:
```bash
helm repo add camunda https://helm.camunda.io
helm repo update
helm template test camunda/camunda-platform \
  --version {chart-version} \
  -f ./generated-values/values-integration-test-ingress-{scenario}.yaml \
  --skip-tests > /dev/null && echo "✅ Valid"
```

---

## Scenario Selection

**Whitelisted scenarios**:

| Scenario | Triggers | Description |
|----------|----------|-------------|
| **`keycloak`** (default) | "SSO", "identity", "full", "production", "all components" | All components (Console, Optimize, WebModeler, Connectors) + embedded Keycloak. Ingress TLS. |
| `keycloak-mt` | "multi-tenant", "tenants", "SaaS" | Multi-tenancy + external Keycloak + external Elasticsearch. Tenant-isolated deployments. |
| `oidc` | "Azure AD", "Entra", "Microsoft", "Okta", "corporate SSO" | External OIDC (preconfigured for Azure AD). No embedded Keycloak. Requires `ENTRA_APP_*` env vars. |
| `opensearch` | "OpenSearch", "AWS", "Amazon" | OpenSearch backend. Elasticsearch disabled. Optimize disabled. Requires `OPENSEARCH_*` env vars. |
| `basic` *(8.8+)* | "simple", "minimal", "quick", "no SSO", "CI/CD" | Orchestration + Connectors only. Basic auth. Console/Identity disabled. Fastest to deploy. |

> 💡 Scenarios sourced from `charts/camunda-platform-{version}/test/integration/scenarios/chart-full-setup/`
---

## Examples

**Production HA**: "Generate values.yaml for prod.company.com with HA, no WebModeler"
```
📋 **Baseline**: `keycloak` scenario
💡 **Why**: "production" + full auth needs SSO/Identity stack.

🔧 **Modifications**:
- `webModeler.enabled: false`
- Orchestration: 3 replicas, replicationFactor: 3
- Elasticsearch: 3 replicas
```

**Enterprise SSO**: "values.yaml with Azure AD for camunda.corp.io"
```
📋 **Baseline**: `oidc` scenario
💡 **Why**: "Azure AD" requires external OIDC configuration.

🔧 **Modifications**:
- ⚠️ Set env: ENTRA_APP_CLIENT_ID, ENTRA_APP_DIRECTORY_ID, ENTRA_APP_CLIENT_SECRET
```

**Multi-tenant SaaS**: "values.yaml for multi-tenant setup at saas.platform.io"
```
📋 **Baseline**: `keycloak-mt` scenario
💡 **Why**: "multi-tenant" requires tenant isolation config.

🔧 **Modifications**:
- None (baseline has multi-tenancy pre-configured)
```
