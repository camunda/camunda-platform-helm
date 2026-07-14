---
name: helm-values-debugging
description: Diagnose Helm values-merge surprises in subchart overrides — Helm's replace-arrays-wholesale merge vs deploy-camunda's name-keyed merge, neutralizing parent-chart array defaults, and Bitnami env-var source chains where duplicate names silently win. Use when a subchart value or env var override is not taking effect, an env var renders twice, or a parent-chart default needs to be removed.
---

# Helm Subchart Value Override Patterns

Helm's values merge is a **deep merge for maps** but a **full replace for arrays**. This matters for subchart overrides:

```yaml
# Parent chart values.yaml default:
elasticsearch:
  master:
    extraEnvVars:           # <-- array
      - name: SOME_VAR
        value: "default"

# Your overlay:
elasticsearch:
  master:
    extraEnvVars: []        # Replaces the entire array — parent default is gone
```

This is the correct way to neutralize a parent chart's default array value. Setting `extraEnvVars: []` removes the parent's entries entirely. Setting `extraEnvVars: [{name: SOME_VAR, value: "override"}]` replaces the array with your single entry.

**Contrast with deploy-camunda's merge:** The `deploy-camunda` CLI uses name-keyed deep merge for `env` arrays (matching on `name` field; `scripts/deploy-camunda/deploy/merge.go`). Entries with matching `name` keys get their values overridden, and new entries are appended — feature layers do NOT need to re-include env vars from `base.yaml`. But Helm itself does NOT do this — Helm replaces arrays wholesale. Know which merge strategy applies at each layer.

# Bitnami Subchart Env Var Chains

Bitnami charts often set env vars from multiple sources in a fixed order within the statefulset template:

1. Security helper (from `security.*` values)
2. `<role>.extraEnvVars` (e.g., `master.extraEnvVars`)
3. Top-level `extraEnvVars`

When Kubernetes encounters duplicate env var names, **the last one wins**. If the parent chart's `values.yaml` defaults an `extraEnvVars` entry that conflicts with a security helper value, you must either override or clear the array. To diagnose:

```bash
# Render and count occurrences of a suspicious env var:
helm template integration charts/camunda-platform-8.X \
  -f <your-values.yaml> \
  --show-only charts/elasticsearch/templates/master/statefulset.yaml \
  | grep -c 'ELASTICSEARCH_ENABLE_REST_TLS'
# Should be exactly 1. If >1, there's a duplicate.
```
