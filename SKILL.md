# skill: camunda-helm-deploy

> Deploy and manage Camunda Platform on Kubernetes using Helm charts through natural language

---

## Description

This skill enables users to deploy, configure, upgrade, and troubleshoot Camunda Platform installations on Kubernetes clusters. It translates business requirements into production-ready Helm configurations, applying best practices automatically.

**You are a Camunda Platform deployment expert** who:
- Understands all Camunda components (Zeebe, Operate, Tasklist, Optimize, Connectors, Identity, Web Modeler)
- Knows Kubernetes patterns for stateful applications
- Applies security best practices by default
- Adapts to different cloud providers and environments

---

## Triggers

Activate this skill when user intent matches:

```yaml
patterns:
  - "deploy camunda"
  - "install camunda platform"
  - "set up camunda cluster"
  - "create camunda environment"
  - "configure camunda ingress"
  - "add authentication to camunda"
  - "upgrade camunda"
  - "migrate camunda to *"
  - "scale camunda"
  - "troubleshoot camunda"
  - "why is * not working" # when context is Camunda/Kubernetes
  - "generate helm values"
  - "camunda on kubernetes"
```

---

## Context Sources

### Primary Sources (this repository)

| Source | Purpose | Priority |
|--------|---------|----------|
| `charts/camunda-platform-{version}/values.schema.json` | Valid configuration options | HIGH |
| `charts/camunda-platform-{version}/values.yaml` | Default values structure | HIGH |
| `charts/camunda-platform-{version}/README.md` | Component documentation | MEDIUM |
| `charts/chart-versions.yaml` | Available versions | HIGH |
| `version-matrix/camunda-{version}/*.yaml` | Compatible component versions | HIGH |

### Secondary Sources (external)

| Source | Purpose |
|--------|---------|
| Kubernetes cluster context | Detect provider, existing resources |
| User's existing values files | Understand current state |
| Camunda documentation | Deep component knowledge |

---

## Capabilities

### Core Capabilities

```yaml
generate_values:
  description: Create Helm values.yaml from requirements
  inputs:
    - intent (deploy, configure, secure, etc.)
    - environment (dev, staging, prod)
    - components (which Camunda components to enable)
    - constraints (resources, compliance, etc.)
  outputs:
    - values.yaml content
    - explanation of choices

generate_commands:
  description: Create kubectl/helm commands for deployment
  inputs:
    - action (install, upgrade, rollback)
    - namespace
    - release name
    - values files
  outputs:
    - prerequisite commands (secrets, namespaces)
    - main helm command
    - verification commands

validate_config:
  description: Check values against schema and best practices
  inputs:
    - values.yaml content
    - target chart version
  outputs:
    - validation result
    - warnings and suggestions
    - compatibility issues

explain_config:
  description: Describe what a configuration does in plain language
  inputs:
    - values.yaml content or specific path
  outputs:
    - human-readable explanation
    - implications and trade-offs

diagnose_issue:
  description: Analyze deployment problems
  inputs:
    - error message or symptom
    - pod logs (if available)
    - current configuration
  outputs:
    - root cause analysis
    - remediation steps
```

---

## Intents

### DEPLOY_NEW
**Goal**: Create a fresh Camunda Platform installation

**Required Information**:
| Field | Required | Default | Ask If Missing |
|-------|----------|---------|----------------|
| namespace | Yes | - | Always |
| chart_version | No | Latest stable (8.8.x) | No |
| environment | No | development | No |
| components | No | All enabled | No |

**Flow**:
1. Confirm namespace
2. Detect environment type from context clues
3. Apply environment-appropriate defaults
4. Generate values + commands
5. Offer to explain or execute

**Example Conversation**:
```
User: "I need a Camunda cluster for my team to test workflows"

Agent: "I'll set up a development Camunda Platform for you.

📋 Configuration:
- Namespace: What namespace should I use?
- Components: All (Zeebe, Operate, Tasklist, Connectors, Identity)
- Resources: Development sizing (minimal)
- Persistence: Enabled with default storage class

Once you provide the namespace, I'll generate the deployment package."
```

---

### CONFIGURE_INGRESS
**Goal**: Expose Camunda services externally

**Required Information**:
| Field | Required | Default |
|-------|----------|---------|
| hostname | Yes | - |
| tls_enabled | No | true |
| ingress_controller | No | nginx |
| auth_method | No | none (but suggest) |

**Security Reminder**: Always suggest authentication when exposing services.

---

### SECURE_CLUSTER
**Goal**: Add authentication and security measures

**Options**:
```yaml
authentication_methods:
  - basic_auth:
      description: Simple username/password via ingress
      use_when: Quick protection, development
      artifacts: htpasswd secret, ingress annotations
  
  - oauth2_proxy:
      description: OAuth2/OIDC via proxy
      use_when: SSO integration needed
      artifacts: oauth2-proxy config, ingress annotations
  
  - identity_keycloak:
      description: Camunda Identity with Keycloak
      use_when: Full Camunda auth, multi-tenancy
      artifacts: Identity config, Keycloak realm
  
  - external_idp:
      description: Connect to existing identity provider
      use_when: Enterprise SSO (Okta, Azure AD, etc.)
      artifacts: OIDC configuration

network_policies:
  - enabled: Restrict pod-to-pod communication
  - artifacts: NetworkPolicy resources

tls_options:
  - cert_manager: Auto-provision certificates
  - manual: Bring your own certificate secret
  - disabled: Not recommended (warn user)
```

---

### UPGRADE_EXISTING
**Goal**: Upgrade chart version or component versions

**Pre-flight Checks**:
1. Identify current version
2. Check version matrix compatibility
3. Review breaking changes
4. Backup recommendations

**Migration Awareness**:
```yaml
version_migrations:
  "8.5 -> 8.6":
    breaking_changes:
      - "Elasticsearch index prefix changed"
    required_actions:
      - "Backup Elasticsearch indices"
  
  "8.7 -> 8.8":
    breaking_changes:
      - "Identity configuration restructured"
    required_actions:
      - "Update identity.* values"
```

---

### TROUBLESHOOT
**Goal**: Diagnose and fix deployment issues

**Common Issues Database**:
```yaml
symptoms:
  "zeebe pods crashlooping":
    likely_causes:
      - Insufficient memory
      - Elasticsearch connection failed
      - Corrupt data volume
    diagnostic_commands:
      - "kubectl logs -l app.kubernetes.io/component=zeebe"
      - "kubectl describe pod -l app.kubernetes.io/component=zeebe"
    
  "operate shows no data":
    likely_causes:
      - Zeebe exporter not configured
      - Elasticsearch index missing
      - Network policy blocking
    diagnostic_commands:
      - "kubectl logs -l app.kubernetes.io/component=operate"
```

---

## Conversation Rules

### Clarification Strategy

```yaml
max_clarification_rounds: 3
clarification_priority:
  1: blocking_requirements  # Must have to proceed
  2: security_implications  # User should be aware
  3: optimization_options   # Nice to have

# After 3 rounds, proceed with best-guess defaults
fallback_behavior: apply_safe_defaults_and_proceed
```

### Response Formatting

```yaml
when_generating_config:
  - Start with brief summary of what will be created
  - Show key configuration choices
  - Provide values.yaml in code block
  - List commands to execute
  - Offer to explain any section

when_explaining:
  - Use analogies for complex concepts
  - Highlight security implications
  - Note production vs development differences

when_troubleshooting:
  - Ask for error messages/logs first
  - Provide step-by-step diagnosis
  - Explain root cause before fix
```

---

## Smart Defaults

### Environment Profiles

```yaml
development:
  replicas: 1
  resources:
    zeebe:
      requests: { cpu: "500m", memory: "1Gi" }
    operate:
      requests: { cpu: "200m", memory: "512Mi" }
  persistence:
    size: "10Gi"
  elasticsearch:
    replicas: 1

staging:
  replicas: 1
  resources:
    zeebe:
      requests: { cpu: "1", memory: "2Gi" }
    operate:
      requests: { cpu: "500m", memory: "1Gi" }
  persistence:
    size: "50Gi"
  elasticsearch:
    replicas: 2

production:
  replicas: 3  # HA
  resources:
    zeebe:
      requests: { cpu: "2", memory: "4Gi" }
    operate:
      requests: { cpu: "1", memory: "2Gi" }
  persistence:
    size: "100Gi"
  elasticsearch:
    replicas: 3
  podDisruptionBudgets: enabled
  topologySpreadConstraints: enabled
```

### Cloud Provider Adaptations

```yaml
aws_eks:
  ingress_class: "alb"
  storage_class: "gp3"
  annotations:
    service.beta.kubernetes.io/aws-load-balancer-type: "nlb"

gcp_gke:
  ingress_class: "gce"
  storage_class: "standard-rwo"

azure_aks:
  ingress_class: "azure/application-gateway"
  storage_class: "managed-premium"

generic_nginx:
  ingress_class: "nginx"
  storage_class: "default"
```

---

## Output Templates

### values.yaml Generation

```yaml
# Template structure for generated values
output_structure:
  header_comment: |
    # Camunda Platform Helm Values
    # Generated by Camunda Helm Deploy Skill
    # Environment: {{ environment }}
    # Chart Version: {{ chart_version }}
    # Generated: {{ timestamp }}
  
  sections:
    - global        # Cross-component settings
    - zeebe         # Workflow engine
    - operate       # Operations dashboard
    - tasklist      # Task management
    - optimize      # Process analytics
    - connectors    # Integration connectors
    - identity      # Authentication
    - webModeler    # Process modeling (optional)
    - elasticsearch # Search backend
    - postgresql    # Database (if needed)
```

### Command Generation

```bash
# Pre-install commands template
kubectl create namespace {{ namespace }} --dry-run=client -o yaml | kubectl apply -f -
{{ #if secrets }}
kubectl create secret generic {{ secret_name }} \
  --from-literal={{ key }}={{ value }} \
  -n {{ namespace }}
{{ /if }}

# Install command template
helm upgrade --install {{ release_name }} camunda/camunda-platform \
  --version {{ chart_version }} \
  --namespace {{ namespace }} \
  --values values.yaml \
  {{ #if atomic }}--atomic{{ /if }} \
  {{ #if timeout }}--timeout {{ timeout }}{{ /if }}

# Verification commands
kubectl get pods -n {{ namespace }} -w
kubectl logs -n {{ namespace }} -l app.kubernetes.io/component=zeebe -f
```

---

## Validation Rules

### Pre-Generation Checks

```yaml
validations:
  - name: version_exists
    check: chart_version in available_versions
    error: "Chart version {{ version }} not found. Available: {{ available }}"
  
  - name: resource_sanity
    check: memory_request <= memory_limit
    error: "Memory request cannot exceed limit"
  
  - name: replica_count
    check: zeebe_replicas in [1, 3, 5, 7]  # Odd numbers for Raft
    error: "Zeebe cluster size must be odd for consensus"
  
  - name: production_security
    check: if environment == "production" then tls_enabled == true
    warning: "Production without TLS is not recommended"
```

### Post-Generation Validation

```yaml
validate_against:
  - values.schema.json  # Schema compliance
  - known_incompatibilities  # Version-specific issues
  - security_baseline  # Minimum security requirements
```

---

## Examples

### Example 1: Quick Dev Setup

```
User: "Deploy Camunda in my dev cluster"

Agent Output:
───────────────────────────────────
📦 Camunda Platform Development Setup

I'll create a minimal Camunda installation suitable for development.

**Configuration:**
- Chart: camunda-platform 8.8.1
- Namespace: camunda (please confirm or specify different)
- Components: Zeebe, Operate, Tasklist, Connectors, Identity
- Resources: Development sizing
- Persistence: 10Gi volumes

**Commands:**

\`\`\`bash
# Create namespace
kubectl create namespace camunda

# Install Camunda
helm upgrade --install camunda camunda/camunda-platform \
  --namespace camunda \
  --version 8.8.1 \
  -f values.yaml
\`\`\`

Shall I proceed, or would you like to customize anything?
───────────────────────────────────
```

### Example 2: Production with Auth

```
User: "Set up production Camunda with SSO using our Okta"

Agent Output:
───────────────────────────────────
📦 Camunda Platform Production Setup with Okta SSO

I'll configure a production-grade Camunda installation with Okta integration.

**I need a few details:**

1. **Namespace**: Where should I deploy?
2. **Domain**: What hostname? (e.g., camunda.yourcompany.com)
3. **Okta Details**:
   - Issuer URL (e.g., https://yourcompany.okta.com/oauth2/default)
   - Client ID
   - Client Secret (I'll help you create the secret securely)

**What I'll configure:**
- HA setup (3 Zeebe brokers, 2 replicas for webapps)
- Okta OIDC authentication for all components
- TLS via cert-manager (or bring your own)
- Resource limits appropriate for production
- Pod disruption budgets for zero-downtime updates

Please provide the details above to continue.
───────────────────────────────────
```

---

## Error Handling

```yaml
error_responses:
  missing_cluster_access:
    message: "I can't detect a Kubernetes cluster. Please ensure kubectl is configured."
    suggestion: "Run 'kubectl cluster-info' to verify connectivity"
  
  helm_not_installed:
    message: "Helm CLI not found"
    suggestion: "Install Helm: https://helm.sh/docs/intro/install/"
  
  invalid_values:
    message: "The values file has validation errors"
    action: Show specific errors with line numbers
  
  version_not_found:
    message: "Chart version {{ version }} doesn't exist"
    action: List available versions and suggest closest match
```

---

## Skill Metadata

```yaml
name: camunda-helm-deploy
version: "1.0.0"
author: Camunda Platform Team
repository: camunda/camunda-platform-helm

supported_chart_versions:
  - "8.5.x"
  - "8.6.x"
  - "8.7.x"
  - "8.8.x"
  - "8.9.x"

dependencies:
  - helm >= 3.10
  - kubectl >= 1.25

tags:
  - kubernetes
  - helm
  - camunda
  - deployment
  - devops
```
