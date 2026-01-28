---
name: camunda-helm-deploy
description: 'Generate Camunda Platform Helm deployments from natural language. Use when asked to deploy Camunda, install Camunda on Kubernetes, set up a Camunda cluster, configure Camunda ingress, generate Helm values, or add authentication to Camunda. Supports development and production environments with nginx ingress and basic auth.'
---

# Camunda Helm Deploy

A skill for generating production-ready Camunda Platform Helm deployments from natural language requests. Transforms user intent into complete values.yaml files and kubectl/helm commands.

## When to Use This Skill

- User asks to "deploy Camunda" or "install Camunda"
- User wants to "set up a Camunda cluster" on Kubernetes
- User needs to "configure ingress" for Camunda
- User asks to "add authentication" or "basic auth" to Camunda
- User wants to "generate Helm values" for Camunda Platform
- User mentions a hostname like "camunda.example.com"

## Prerequisites

- Kubernetes cluster access (`kubectl` configured)
- Helm 3.10+ installed
- For basic auth: `htpasswd` command available

## Step-by-Step Workflow

### Step 1: Parse User Intent

Extract from user message:

| Pattern | Meaning | Default |
|---------|---------|---------|
| "namespace X", "ns X" | Custom namespace | `camunda` |
| hostname mentioned | Enable ingress | - |
| "ingress", "expose" | Enable ingress | disabled |
| "basic auth", "authentication" | Enable basic auth | disabled |
| "production", "prod" | HA sizing | development |
| "no tls", "without tls" | Disable TLS | enabled |

### Step 2: Generate Configuration Summary

Present a table:

| Setting | Value |
|---------|-------|
| Namespace | `<namespace>` |
| Chart Version | `13.4.1` (Camunda 8.8.x) |
| Environment | development/production |
| Hostname | `<hostname>` |
| Ingress | ✅/❌ |
| Basic Auth | ✅/❌ |
| TLS | ✅/❌ |

### Step 3: Generate values.yaml

Use the helper script or knowledge.json for accurate values:

```bash
node .github/skills/camunda-helm-deploy/generate-values.js "<user request>"
```

#### Development Defaults
```yaml
zeebe:
  clusterSize: 1
  partitionCount: 1
  replicationFactor: 1
elasticsearch:
  replicas: 1
```

#### Production Defaults
```yaml
zeebe:
  clusterSize: 3
  partitionCount: 3
  replicationFactor: 3
elasticsearch:
  replicas: 3
  minimumMasterNodes: 2
```

#### Ingress with Basic Auth
```yaml
global:
  ingress:
    enabled: true
    className: nginx
    host: "<hostname>"
    annotations:
      nginx.ingress.kubernetes.io/auth-type: basic
      nginx.ingress.kubernetes.io/auth-secret: camunda-basic-auth
      nginx.ingress.kubernetes.io/auth-realm: "Authentication Required"
    tls:
      enabled: true
      secretName: camunda-platform-tls
```

### Step 4: Generate Commands

Provide in order:

1. **Helm repo**:
```bash
helm repo add camunda https://helm.camunda.io
helm repo update
```

2. **Namespace**:
```bash
kubectl create namespace <namespace> --dry-run=client -o yaml | kubectl apply -f -
```

3. **Basic auth secret** (if enabled):
```bash
htpasswd -cb auth USERNAME PASSWORD
kubectl create secret generic camunda-basic-auth --from-file=auth -n <namespace>
rm auth
```

4. **TLS secret** (if enabled, not using cert-manager):
```bash
kubectl create secret tls camunda-platform-tls \
  --cert=path/to/tls.crt \
  --key=path/to/tls.key \
  -n <namespace>
```

5. **Helm install**:
```bash
helm upgrade --install camunda camunda/camunda-platform \
  --namespace <namespace> \
  --version 13.4.1 \
  -f values.yaml \
  --wait
```

6. **Verify**:
```bash
kubectl get pods -n <namespace> -w
```

### Step 5: Provide Access URLs

```
- Operate: https://<hostname>/operate
- Tasklist: https://<hostname>/tasklist  
- Optimize: https://<hostname>/optimize
```

## Troubleshooting

| Issue | Solution |
|-------|----------|
| Pods not starting | Check resources: `kubectl describe pod -n <ns>` |
| Ingress not working | Verify ingress controller: `kubectl get ingressclass` |
| Auth not prompting | Check secret exists: `kubectl get secret camunda-basic-auth -n <ns>` |
| TLS errors | Verify cert/key match: `openssl verify` |

## References

- Chart source: `charts/camunda-platform-8.8/`
- Values schema: `charts/camunda-platform-8.8/values.schema.json`
- Full values: `charts/camunda-platform-8.8/values.yaml`
- Helm repo: https://helm.camunda.io
