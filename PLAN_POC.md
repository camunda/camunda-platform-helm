# PLAN_POC.md - Half-Day Proof of Concept

## Objective

**In 4 hours**, demonstrate an AI agent that can take a natural language request and generate a working Camunda Platform Helm deployment.

**Demo Scenario**: *"Deploy Camunda with ingress and basic auth on my cluster"*

---

## Scope

### ✅ In Scope (PoC)
- Single intent: `DEPLOY_NEW` with ingress + basic auth
- One chart version: `8.8.x` (latest)
- One ingress controller: `nginx`
- Generate: `values.yaml` + shell commands
- Hardcoded smart defaults

### ❌ Out of Scope (Future)
- Multiple intents (upgrade, troubleshoot, scale)
- Version selection/migration
- Cloud provider detection
- Cluster state awareness
- External IdP integration

---

## Time Budget (4 hours)

| Phase | Duration | Deliverable |
|-------|----------|-------------|
| 1. Setup | 30 min | MCP tool skeleton, test harness |
| 2. Knowledge | 45 min | Extract key config from 8.8 schema |
| 3. Core Logic | 90 min | Intent parser + values generator |
| 4. Output | 45 min | Format values.yaml + commands |
| 5. Demo | 30 min | End-to-end test + polish |

---

## Phase 1: Setup (30 min)

### 1.1 Create MCP Tool Structure

```
.github/copilot/
├── vscode-mcp.json          # Tool registration
└── skills/
    └── camunda-deploy/
        ├── tool.js          # Main entry point
        ├── knowledge.json   # Extracted chart knowledge
        └── templates/
            └── values.yaml.hbs  # Output template
```

### 1.2 Register Tool in vscode-mcp.json

```json
{
  "tools": {
    "camunda-deploy": {
      "description": "Generate Camunda Platform Helm deployment",
      "entrypoint": ".github/copilot/skills/camunda-deploy/tool.js"
    }
  }
}
```

---

## Phase 2: Knowledge Extraction (45 min)

### 2.1 Extract Minimal Schema

From `charts/camunda-platform-8.8/values.schema.json`, extract only:

```json
{
  "chartVersion": "8.8.1",
  "components": ["zeebe", "operate", "tasklist", "connectors", "identity"],
  "ingressOptions": {
    "controllers": ["nginx"],
    "authMethods": ["basic", "none"],
    "tlsOptions": ["enabled", "disabled"]
  },
  "defaults": {
    "development": { /* minimal resources */ },
    "production": { /* HA resources */ }
  }
}
```

### 2.2 Create knowledge.json

Hardcode the essential mappings:
- Component → values paths
- Ingress controller → annotations
- Auth method → secrets + annotations

---

## Phase 3: Core Logic (90 min)

### 3.1 Intent Parser (Simple)

```javascript
function parseIntent(userMessage) {
  const intent = {
    action: 'deploy',
    namespace: extractNamespace(userMessage) || 'camunda',
    hostname: extractHostname(userMessage) || null,
    ingress: userMessage.includes('ingress'),
    basicAuth: userMessage.includes('basic auth') || userMessage.includes('authentication'),
    tls: !userMessage.includes('no tls'),
    environment: userMessage.includes('production') ? 'production' : 'development'
  };
  
  return intent;
}
```

### 3.2 Values Generator

```javascript
function generateValues(intent, knowledge) {
  const values = {
    global: {
      ingress: {
        enabled: intent.ingress,
        host: intent.hostname || 'camunda.example.com',
        tls: { enabled: intent.tls }
      }
    },
    // ... component configs from knowledge.json
  };
  
  if (intent.basicAuth) {
    values.global.ingress.annotations = {
      'nginx.ingress.kubernetes.io/auth-type': 'basic',
      'nginx.ingress.kubernetes.io/auth-secret': 'camunda-basic-auth',
      'nginx.ingress.kubernetes.io/auth-realm': 'Authentication Required'
    };
  }
  
  return values;
}
```

### 3.3 Command Generator

```javascript
function generateCommands(intent, values) {
  const commands = [];
  
  // Namespace
  commands.push(`kubectl create namespace ${intent.namespace} --dry-run=client -o yaml | kubectl apply -f -`);
  
  // Basic auth secret
  if (intent.basicAuth) {
    commands.push(`# Create basic auth secret (replace USER and PASSWORD)`);
    commands.push(`htpasswd -cb auth USER PASSWORD`);
    commands.push(`kubectl create secret generic camunda-basic-auth --from-file=auth -n ${intent.namespace}`);
  }
  
  // Helm install
  commands.push(`helm upgrade --install camunda camunda/camunda-platform \\
  --namespace ${intent.namespace} \\
  --version 8.8.1 \\
  -f values.yaml`);
  
  return commands;
}
```

---

## Phase 4: Output Formatting (45 min)

### 4.1 Response Template

```markdown
## 📦 Camunda Platform Deployment

**Configuration Summary:**
- Namespace: `{{ namespace }}`
- Hostname: `{{ hostname }}`
- Ingress: {{ ingress ? '✅ Enabled' : '❌ Disabled' }}
- Basic Auth: {{ basicAuth ? '✅ Enabled' : '❌ Disabled' }}
- TLS: {{ tls ? '✅ Enabled' : '❌ Disabled' }}
- Environment: {{ environment }}

### Step 1: Create values.yaml

\`\`\`yaml
{{ values }}
\`\`\`

### Step 2: Run Commands

\`\`\`bash
{{ commands }}
\`\`\`

### Step 3: Verify

\`\`\`bash
kubectl get pods -n {{ namespace }} -w
\`\`\`
```

---

## Phase 5: Demo & Polish (30 min)

### 5.1 Test Scenarios

| Input | Expected Output |
|-------|-----------------|
| "Deploy Camunda" | Basic install, no ingress |
| "Deploy Camunda with ingress" | Ingress enabled, no auth |
| "Deploy Camunda with ingress and basic auth" | Full output with secrets |
| "Production Camunda cluster at camunda.mycompany.com" | HA config, TLS, hostname set |

### 5.2 Demo Script

```
1. Show empty cluster (kubectl get pods)
2. Ask agent: "Deploy Camunda with ingress and basic auth at demo.camunda.io"
3. Show generated values.yaml
4. Run the commands
5. Watch pods come up
6. Access UI with auth
```

---

## File Structure (Final)

```
.github/copilot/skills/camunda-deploy/
├── tool.js              # ~100 lines - main logic
├── knowledge.json       # ~50 lines - extracted from schema
├── templates/
│   └── response.md.hbs  # ~40 lines - output template
└── README.md            # How to use
```

---

## Success Criteria

| Criteria | Target |
|----------|--------|
| Parse basic deploy intent | ✅ Works |
| Generate valid values.yaml | ✅ Helm lint passes |
| Include auth configuration | ✅ Annotations correct |
| Generate runnable commands | ✅ Copy-paste works |
| Total lines of code | < 200 |

---

## Stretch Goals (if time permits)

1. **Add hostname validation** - Check if valid domain format
2. **Add TLS secret command** - Generate cert-manager or manual TLS
3. **Component selection** - "Deploy Camunda without Optimize"
4. **Dry-run mode** - Show what would happen without generating

---

## Quick Start

```bash
# 1. Create the skill directory
mkdir -p .github/copilot/skills/camunda-deploy/templates

# 2. Copy the knowledge from chart
cat charts/camunda-platform-8.8/values.yaml | head -100 > reference.yaml

# 3. Start building tool.js
code .github/copilot/skills/camunda-deploy/tool.js
```

---

## Notes

- Keep it simple: No fancy parsing, regex is fine
- Hardcode where possible: Only 8.8, only nginx
- Output matters: Make the response look professional
- Test early: Try the generated values with `helm template`
