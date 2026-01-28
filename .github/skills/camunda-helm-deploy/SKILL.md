---
name: camunda-helm-deploy
description: 'Generate Camunda Platform Helm deployments from natural language. Uses battle-tested integration scenarios as baselines and applies user-specific modifications. Supports Keycloak SSO, basic auth, OIDC, OpenSearch, and multi-tenancy.'
---

# Camunda Helm Deploy

Generate production-ready Camunda Platform Helm values from natural language. Uses real integration test scenarios as validated baselines, then applies targeted modifications.

## Triggers

Activate when user mentions:
- "deploy camunda", "install camunda", "set up camunda"
- "camunda on kubernetes", "helm values"
- A hostname like "camunda.example.com"
- Authentication: "keycloak", "basic auth", "OIDC", "Azure AD"

## Prerequisites

- `kubectl` configured with cluster access
- `helm` 3.10+ installed
- Repository cloned: `camunda/camunda-platform-helm`
- Go tools built: `make install.prepare-helm-values`

---

## Workflow

### Step 1: Select Scenario + Explain

**ALWAYS output this block** to show transparency:

```
📋 **Baseline**: `{scenario}` scenario
💡 **Why**: {1 sentence}

🔧 **Modifications**:
- {change 1}
- {change 2}
```

### Step 2: Generate Baseline

```bash
export CAMUNDA_HOSTNAME="{hostname}"
prepare-helm-values \
  --chart-path ./charts/camunda-platform-8.9 \
  --scenario {scenario} \
  --output-dir /tmp/camunda-values \
  --interactive=false
```

### Step 3: Apply Modifications

Edit the generated file at `/tmp/camunda-values/values-integration-test-ingress-{scenario}.yaml`

### Step 4: Validate

```bash
helm template test ./charts/camunda-platform-8.9 \
  -f /tmp/camunda-values/values-integration-test-ingress-{scenario}.yaml \
  --skip-tests
```

### Step 5: Output Commands

```bash
# Create namespace
kubectl create namespace {namespace} --dry-run=client -o yaml | kubectl apply -f -

# Install
helm upgrade --install camunda ./charts/camunda-platform-8.9 \
  --namespace {namespace} \
  -f values.yaml \
  --wait
```

---

## Scenario Selection

| Scenario | Triggers | Description |
|----------|----------|-------------|
| **`keycloak`** (default) | "deploy", "SSO", "identity", "full" | Full stack + embedded Keycloak |
| `basic` | "simple", "basic auth", "minimal", "no SSO" | Basic auth, no Identity/Keycloak |
| `oidc` | "Azure AD", "Entra", "Okta", "external IdP" | External OIDC provider |
| `opensearch` | "OpenSearch", "AWS" | OpenSearch backend |
| `keycloak-mt` | "multi-tenant", "tenants" | Multi-tenancy enabled |

### Scenario Details

#### `keycloak` (default)
- **Components**: All enabled (Orchestration, Console, Optimize, WebModeler, Connectors)
- **Auth**: Embedded Keycloak + Identity
- **Placeholders**: `$CAMUNDA_HOSTNAME`
- **Best for**: Production-like setups with full SSO

#### `basic`
- **Components**: Orchestration only, Console disabled
- **Auth**: Basic authentication at orchestration level
- **Placeholders**: `$CAMUNDA_HOSTNAME`
- **Best for**: Quick testing, CI/CD pipelines

#### `oidc`
- **Components**: All enabled
- **Auth**: External OIDC (Azure AD/Entra)
- **Placeholders**: `$CAMUNDA_HOSTNAME`, `$ENTRA_APP_*` variables
- **Best for**: Enterprise SSO integration

#### `opensearch`
- **Components**: Orchestration-focused
- **Auth**: Keycloak (from overlay)
- **Placeholders**: `$OPENSEARCH_*` variables
- **Best for**: AWS environments

---

## Modifications Reference

### Environment Sizing

**Development** (default):
```yaml
# No changes needed - scenarios use sensible defaults
```

**Production**:
```yaml
zeebe:
  clusterSize: 3
  partitionCount: 3
  replicationFactor: 3
  resources:
    requests:
      cpu: "1"
      memory: "2Gi"
    limits:
      cpu: "2"
      memory: "4Gi"

elasticsearch:
  replicas: 3
  minimumMasterNodes: 2
```

### Disable Components

```yaml
optimize:
  enabled: false

webModeler:
  enabled: false

console:
  enabled: false

connectors:
  enabled: false
```

### Custom Resources

```yaml
orchestration:
  resources:
    requests:
      cpu: "500m"
      memory: "1Gi"
    limits:
      cpu: "1"
      memory: "2Gi"
```

---

## Response Examples

### Example 1: Simple Request

**User**: "Deploy Camunda at demo.camunda.io"

```
📋 **Baseline**: `keycloak` scenario
💡 **Why**: Default full stack with embedded Identity/Keycloak for SSO.

🔧 **Modifications**:
- None (using baseline as-is)
```

### Example 2: Development Setup

**User**: "Deploy Camunda for dev at dev.example.com, no Optimize"

```
📋 **Baseline**: `keycloak` scenario
💡 **Why**: Full stack requested (dev doesn't imply minimal).

🔧 **Modifications**:
- Disabled Optimize (`optimize.enabled: false`)
```

### Example 3: Basic Auth

**User**: "Simple Camunda cluster with basic auth at test.local"

```
📋 **Baseline**: `basic` scenario
💡 **Why**: "Simple" + "basic auth" → minimal setup without Identity overhead.

🔧 **Modifications**:
- None (basic scenario already configured for basic auth)
```

### Example 4: Production HA

**User**: "Production Camunda with HA at prod.company.com"

```
📋 **Baseline**: `keycloak` scenario
💡 **Why**: Production with full auth capabilities.

🔧 **Modifications**:
- Zeebe: 3 replicas, increased resources
- Elasticsearch: 3 replicas
- Added PodDisruptionBudgets
```

### Example 5: Azure AD

**User**: "Camunda with Azure AD SSO at azure.mycompany.com"

```
📋 **Baseline**: `oidc` scenario
💡 **Why**: Azure AD requires external OIDC configuration.

🔧 **Modifications**:
- Set hostname
- ⚠️ Requires env vars: `ENTRA_APP_CLIENT_ID`, `ENTRA_APP_DIRECTORY_ID`, `ENTRA_APP_CLIENT_SECRET`
```

---

## Validation

Always validate before presenting to user:

```bash
# Syntax check
helm template test ./charts/camunda-platform-8.9 \
  -f values.yaml \
  --skip-tests > /dev/null && echo "✅ Valid"

# Schema check (optional)
helm lint ./charts/camunda-platform-8.9 -f values.yaml
```

---

## Troubleshooting

| Issue | Solution |
|-------|----------|
| Missing env var error | Set required `$CAMUNDA_HOSTNAME` or scenario-specific vars |
| Helm template fails | Check YAML syntax, ensure all placeholders substituted |
| Pods not starting | `kubectl describe pod -n <ns>` for events |
| Ingress not working | Verify ingress controller: `kubectl get ingressclass` |

---

## References

- Scenarios: `charts/camunda-platform-8.9/test/integration/scenarios/chart-full-setup/`
- Values schema: `charts/camunda-platform-8.9/values.schema.json`
- prepare-helm-values: `scripts/prepare-helm-values/`
- Helm repo: https://helm.camunda.io
