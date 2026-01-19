# Integration Test Scenario Resolution

How the Go CLI (`deploy-camunda`) resolves Helm values files for each integration
test scenario, from the CI config entry to the final `helm install -f` arguments.

---

## Table of Contents

- [Architecture Overview](#architecture-overview)
- [Data Flow](#data-flow)
- [Layer Resolution Order](#layer-resolution-order)
- [Scenario Name Parsing (MapScenarioToConfig)](#scenario-name-parsing-mapscenariotoconfig)
- [Selection + Composition Override](#selection--composition-override)
- [Available Layer Files by Version](#available-layer-files-by-version)
- [Scenario Resolution Per Version](#scenario-resolution-per-version)
  - [8.2 and 8.3](#82-and-83)
  - [8.4 and 8.5](#84-and-85)
  - [8.6](#86)
  - [8.7](#87)
  - [8.8](#88)
  - [8.9](#89)

---

## Architecture Overview

Each integration test scenario produces an ordered list of Helm values files that
are passed to `helm install -f`. The system uses a **layered values** approach:
a base configuration is extended by identity, persistence, platform, and feature
layers, then optionally by migrator and image-tag overrides.

There are two configuration approaches in `ci-test-config.yaml`:

1. **Legacy (name parsing)** — The scenario `name` is parsed by
   `MapScenarioToConfig()` to derive identity, persistence, platform, and
   features. Example: `name: keycloak-mt` → identity=`keycloak-external`,
   features=`[multitenancy]`.

2. **Selection + Composition (explicit)** — The scenario entry includes
   explicit `identity`, `persistence`, and `features` fields that override
   name parsing. Example:
   ```yaml
   name: keycloak-original
   identity: keycloak-external
   persistence: elasticsearch-external
   ```

Both approaches produce a `DeploymentConfig` that is resolved into file paths.

---

## Data Flow

```
ci-test-config.yaml
        │
        ▼
   CIScenario struct           (matrix/config.go)
        │
        ▼
   Entry struct                 (matrix/matrix.go — Generate())
   ├── Identity, Persistence,   (propagated from CIScenario)
   │   Features fields
   ├── Version, Flow, Platform
   └── Scenario, Shortname, Auth, Exclude
        │
        ▼
   RuntimeFlags                 (matrix/runner.go — executeEntry())
   ├── Identity, Persistence,   (set from Entry fields)
   │   Features
   └── all other deploy flags
        │
        ▼
   deploy.Execute()             (deploy/deploy.go)
        │
        ▼
   prepareScenarioValues()
   ├── MapScenarioToConfig(scenarioName)   → DeploymentConfig (name-parsed)
   ├── Apply flag overrides:                → DeploymentConfig (final)
   │   if flags.Identity != ""    → deployConfig.Identity = flags.Identity
   │   if flags.Persistence != "" → deployConfig.Persistence = flags.Persistence
   │   if len(flags.Features) > 0 → deployConfig.Features = flags.Features
   │   if flags.TestPlatform != ""→ deployConfig.Platform = flags.TestPlatform
   └── deployConfig.ResolvePaths(scenarioDir) → []string (ordered file list)
        │
        ▼
   helm install -f common1.yaml -f common2.yaml -f layer1.yaml -f layer2.yaml ...
```

**Key rule:** Explicit `identity`/`persistence`/`features` fields in
ci-test-config.yaml always override name-based derivation.

---

## Layer Resolution Order

`ResolvePaths()` returns files in this order
(see `scripts/camunda-core/pkg/scenarios/scenarios.go:83`):

| Order | Layer | File(s) | Condition |
|-------|-------|---------|-----------|
| 1 | Base | `values/base.yaml` | Always (required) |
| 2 | QA modifier | `values/base-qa.yaml` | `QA == true` |
| 2 | Upgrade modifier | `values/base-upgrade.yaml` | `Upgrade == true` |
| 3 | Identity | `values/identity/<identity>.yaml` | `Identity != ""` |
| 4 | Persistence | `values/persistence/<persistence>.yaml` | `Persistence != ""` |
| 5 | Platform | `values/platform/<platform>.yaml` | `Platform != ""` |
| 6 | Features | `values/features/<feature>.yaml` | For each feature, in order: `multitenancy`, `rba`, `documentstore` |
| 7 | Migrator | `values-enable-migrator.yaml` | Chart version starts with `13` AND flow != `upgrade-patch` |
| 8 | Image tags | `values/base-image-tags.yaml` | `ImageTags == true` |

Later files override earlier ones (standard Helm `-f` precedence).

---

## Scenario Name Parsing (MapScenarioToConfig)

When no explicit `identity`/`persistence`/`features` fields are set, the
scenario name is parsed by `MapScenarioToConfig()` using substring matching:

### Identity derivation

| Scenario name contains | Identity assigned |
|------------------------|-------------------|
| `keycloak-original` (exact) | `keycloak-external` (early return) |
| `keycloak-mt`, `-mt-`, `multitenancy` | `keycloak-external` |
| `oidc`, `entra` | `oidc` |
| `basic` | `basic` |
| `hybrid` | `hybrid` |
| _(default)_ | `keycloak` |

### Persistence derivation

| Scenario name contains | Persistence assigned |
|------------------------|----------------------|
| `keycloak-original` (exact) | `elasticsearch-external` (early return) |
| `opensearch` | `opensearch` |
| `rdbms` + `oracle` | `rdbms-oracle` |
| `rdbms` | `rdbms` |
| _(default)_ | `elasticsearch` |

### Platform derivation

| Scenario name contains | Platform assigned |
|------------------------|-------------------|
| `eks` | `eks` |
| `openshift`, `rosa` | `openshift` |
| _(default)_ | `gke` |

### Feature derivation

| Scenario name contains | Feature added |
|------------------------|---------------|
| `-mt`, `multitenancy` | `multitenancy` |
| `rba` | `rba` |
| `document` | `documentstore` |

### Other flags

| Scenario name contains | Flag set |
|------------------------|----------|
| `qa-` prefix | `QA = true` |
| `upgrade`, `-upg` | `Upgrade = true` |

---

## Selection + Composition Override

When a ci-test-config.yaml entry has explicit fields, these override the
name-parsed values. The override happens in `prepareScenarioValues()`
(`deploy/deploy.go:1063-1073`):

```go
// MapScenarioToConfig runs first (name parsing)
deployConfig := scenarios.MapScenarioToConfig(scenarioCtx.ScenarioName)

// Then explicit fields override:
if flags.Identity != ""       { deployConfig.Identity = flags.Identity }
if flags.Persistence != ""    { deployConfig.Persistence = flags.Persistence }
if len(flags.Features) > 0    { deployConfig.Features = flags.Features }
```

This means a scenario like:
```yaml
name: keycloak-original
identity: keycloak-external
persistence: elasticsearch-external
```
...first parses `keycloak-original` (which already returns the correct values
via the early-return path), then applies the explicit overrides (which happen to
match). The explicit fields serve as a safety net and documentation.

---

## Available Layer Files by Version

Versions 8.2–8.5 use the legacy (non-layered) values system and do not have a
`values/` subdirectory structure. The layered system applies to 8.6+.

### Legend

- &#x2713; = file exists
- &#x2717; = file does not exist

| File | 8.6 | 8.7 | 8.8 | 8.9 |
|------|-----|-----|-----|-----|
| **Base** | | | | |
| `base.yaml` | &#x2713; | &#x2713; | &#x2713; | &#x2713; |
| `base-qa.yaml` | &#x2713; | &#x2713; | &#x2713; | &#x2713; |
| `base-upgrade.yaml` | &#x2717; | &#x2713; | &#x2713; | &#x2713; |
| `base-image-tags.yaml` | &#x2713; | &#x2713; | &#x2713; | &#x2713; |
| **Identity** | | | | |
| `identity/keycloak.yaml` | &#x2713; | &#x2713; | &#x2713; | &#x2713; |
| `identity/keycloak-external.yaml` | &#x2713; | &#x2713; | &#x2713; | &#x2713; |
| `identity/oidc.yaml` | &#x2717; | &#x2713; | &#x2713; | &#x2713; |
| `identity/basic.yaml` | &#x2717; | &#x2717; | &#x2713; | &#x2713; |
| `identity/hybrid.yaml` | &#x2717; | &#x2717; | &#x2713; | &#x2713; |
| **Persistence** | | | | |
| `persistence/elasticsearch.yaml` | &#x2713; | &#x2713; | &#x2713; | &#x2713; |
| `persistence/elasticsearch-external.yaml` | &#x2713; | &#x2713; | &#x2713; | &#x2713; |
| `persistence/opensearch.yaml` | &#x2713; | &#x2713; | &#x2713; | &#x2713; |
| `persistence/rdbms.yaml` | &#x2717; | &#x2717; | &#x2717; | &#x2713; |
| `persistence/rdbms-oracle.yaml` | &#x2717; | &#x2717; | &#x2717; | &#x2713; |
| **Platform** | | | | |
| `platform/gke.yaml` | &#x2713; | &#x2713; | &#x2713; | &#x2713; |
| `platform/eks.yaml` | &#x2713; | &#x2713; | &#x2713; | &#x2713; |
| **Features** | | | | |
| `features/multitenancy.yaml` | &#x2713; | &#x2713; | &#x2713; | &#x2713; |
| `features/rba.yaml` | &#x2713; | &#x2713; | &#x2713; | &#x2713; |
| `features/documentstore.yaml` | &#x2717; | &#x2713; | &#x2713; | &#x2713; |

---

## Scenario Resolution Per Version

### 8.2 and 8.3

These versions use the **legacy (non-layered) values system**. They have a single
`base` scenario and no `case`/`scenario` structure in ci-test-config.yaml.

```yaml
# ci-test-config.yaml structure:
integration:
  scenarios:
    pr:
      - name: base
        enabled: true
```

There is no layered `values/` directory. The legacy code path
(`processValues()` + `BuildValuesList()`) is used. The Go CLI detects this via
`scenarios.HasLayeredValues()` returning `false`.

**Resolved values:** determined entirely by the legacy `BuildValuesList()` logic.

---

### 8.4 and 8.5

First versions to use the `case`/`scenario` structure with `auth`, `shortname`,
and `exclude` fields. Still use the **legacy values system** (no layered
`values/` directory).

#### 8.4 Scenarios

| Tier | Scenario | Auth | Flow | Platforms |
|------|----------|------|------|-----------|
| PR | `elasticsearch` (es) | keycloak | _(all)_ | _(default)_ |
| Nightly | `elasticsearch` (es) | keycloak | _(all)_ | _(default)_ |
| Nightly | `multitenancy` (mt) | keycloak | _(all)_ | _(default)_ |

#### 8.5 Scenarios

| Tier | Scenario | Auth | Flow | Platforms |
|------|----------|------|------|-----------|
| PR | `elasticsearch` (es) | keycloak | install, upgrade-patch | _(default)_ |
| Nightly | `elasticsearch` (es) | keycloak | _(all)_ | _(default)_ |
| Nightly | `multitenancy` (mt) | keycloak | _(all)_ | _(default)_ |

**Resolved values:** Legacy path. The `auth` field selects which auth values
file to process.

---

### 8.6

First version with the **layered values system**. The `values/` directory has
identity, persistence, platform, and features subdirectories.

#### Scenarios

| Tier | Scenario | Shortname | Auth | Flow | Platforms | Name-Parsed Config |
|------|----------|-----------|------|------|-----------|--------------------|
| PR | `elasticsearch` | es | keycloak | install, upgrade-patch | gke, eks | identity=`keycloak`, persistence=`elasticsearch`, platform=per-entry |
| Nightly | `elasticsearch` | es | keycloak | _(all)_ | _(default=gke)_ | identity=`keycloak`, persistence=`elasticsearch`, platform=`gke` |
| Nightly | `multitenancy` | mt | keycloak | _(all)_ | _(default=gke)_ | identity=`keycloak-external`, persistence=`elasticsearch`, platform=`gke`, features=`[multitenancy]` |
| Nightly | `opensearch` | os | keycloak | _(all)_ | _(default=gke)_ | identity=`keycloak`, persistence=`opensearch`, platform=`gke` |

None of these scenarios have explicit `identity`/`persistence`/`features` fields,
so name parsing is the sole source of config.

#### Resolved Values Files

**`elasticsearch` on GKE:**
```
values/base.yaml
values/identity/keycloak.yaml
values/persistence/elasticsearch.yaml
values/platform/gke.yaml
```

**`elasticsearch` on EKS:**
```
values/base.yaml
values/identity/keycloak.yaml
values/persistence/elasticsearch.yaml
values/platform/eks.yaml
```

**`multitenancy` on GKE:**
```
values/base.yaml
values/identity/keycloak-external.yaml
values/persistence/elasticsearch.yaml
values/platform/gke.yaml
values/features/multitenancy.yaml
```

**`opensearch` on GKE:**
```
values/base.yaml
values/identity/keycloak.yaml
values/persistence/opensearch.yaml
values/platform/gke.yaml
```

#### What 8.6 Does NOT Have
- No `oidc.yaml`, `basic.yaml`, `hybrid.yaml` identity layers
- No `base-upgrade.yaml`
- No `documentstore.yaml` feature

#### Note on `elasticsearch-external.yaml`
The `persistence/elasticsearch-external.yaml` file exists on 8.6 for
consistency, but no scenario in ci-test-config.yaml currently uses it.
The `multitenancy` scenario uses `keycloak-external` identity with internal
`elasticsearch` persistence (same as 8.7+).

---

### 8.7

Adds OIDC identity, external Elasticsearch persistence, document store feature,
and upgrade support.

#### Scenarios

| Tier | Scenario | Shortname | Auth | Flow | Platforms | Explicit Fields | Name-Parsed Config |
|------|----------|-----------|------|------|-----------|-----------------|---------------------|
| PR | `elasticsearch` | es | keycloak | install, upgrade-patch | gke, eks | — | identity=`keycloak`, persistence=`elasticsearch` |
| PR | `keycloak-mt` | kemt | keycloak | install, upgrade-patch | _(gke)_ | — | identity=`keycloak-external`, persistence=`elasticsearch`, features=`[multitenancy]` |
| PR | `oidc` | esoi | oidc | install, upgrade-patch | _(gke)_ | — | identity=`oidc`, persistence=`elasticsearch` |
| PR | `keycloak-original` | keyc | keycloak | install, upgrade-patch | _(gke)_ | identity=`keycloak-external`, persistence=`elasticsearch-external` | identity=`keycloak-external`, persistence=`elasticsearch-external` (override matches early-return) |
| Nightly | `elasticsearch` | es | keycloak | _(all)_ | _(gke)_ | — | identity=`keycloak`, persistence=`elasticsearch` |
| Nightly | `multitenancy` | mt | keycloak | _(all)_ | _(gke)_ | — | identity=`keycloak-external`, persistence=`elasticsearch`, features=`[multitenancy]` |
| Nightly | `oidc` | oi | keycloak | _(all)_ | _(gke)_ | — | identity=`oidc`, persistence=`elasticsearch` |
| Nightly | `opensearch` | os | keycloak | _(all)_ | _(gke)_ | — | identity=`keycloak`, persistence=`opensearch` |
| Nightly | `elasticsearch` (enterprise) | — | keycloak | _(all)_ | _(gke)_ | — | identity=`keycloak`, persistence=`elasticsearch` |
| Nightly | `documentstore` | docstr | keycloak | _(all)_ | _(gke)_ | — | identity=`keycloak`, persistence=`elasticsearch`, features=`[documentstore]` |

Note: `keycloak-mt` (PR) is **disabled** (`enabled: false`).

#### Resolved Values Files

**`elasticsearch` on GKE:**
```
values/base.yaml
values/identity/keycloak.yaml
values/persistence/elasticsearch.yaml
values/platform/gke.yaml
```

**`keycloak-mt` on GKE:**
```
values/base.yaml
values/identity/keycloak-external.yaml
values/persistence/elasticsearch.yaml
values/platform/gke.yaml
values/features/multitenancy.yaml
```

**`oidc` on GKE:**
```
values/base.yaml
values/identity/oidc.yaml
values/persistence/elasticsearch.yaml
values/platform/gke.yaml
```

**`keycloak-original` on GKE:**
```
values/base.yaml
values/identity/keycloak-external.yaml
values/persistence/elasticsearch-external.yaml
values/platform/gke.yaml
```

**`opensearch` on GKE:**
```
values/base.yaml
values/identity/keycloak.yaml
values/persistence/opensearch.yaml
values/platform/gke.yaml
```

**`documentstore` on GKE:**
```
values/base.yaml
values/identity/keycloak.yaml
values/persistence/elasticsearch.yaml
values/platform/gke.yaml
values/features/documentstore.yaml
```

**`multitenancy` on GKE:**
```
values/base.yaml
values/identity/keycloak-external.yaml
values/persistence/elasticsearch.yaml
values/platform/gke.yaml
values/features/multitenancy.yaml
```

#### What 8.7 Adds Over 8.6
- `identity/oidc.yaml` — Microsoft Entra ID configuration
- `persistence/elasticsearch-external.yaml` — shared CI Elasticsearch cluster
- `features/documentstore.yaml` — AWS S3 document store
- `base-upgrade.yaml` — Elasticsearch availability during upgrades

#### What 8.7 Does NOT Have
- No `identity/basic.yaml` or `identity/hybrid.yaml`

---

### 8.8

Major restructuring: zeebe, operate, and tasklist are unified into a single
`orchestration` component. Adds basic and hybrid identity modes.

#### Scenarios

| Tier | Scenario | Shortname | Auth | Flow | Platforms | Explicit Fields | Resolved Identity | Resolved Persistence | Resolved Features |
|------|----------|-----------|------|------|-----------|-----------------|-------------------|----------------------|-------------------|
| PR | `elasticsearch` | eske | keycloak | _(all)_ | gke, eks | — | `keycloak` | `elasticsearch` | — |
| PR | `upgrade-migration` | eske-upgm | keycloak | upgrade-minor | _(gke)_ | — | `keycloak` | `elasticsearch` | — |
| PR | `elasticsearch-basic` | esba | basic | install | _(gke)_ | — | `basic` | `elasticsearch` | — |
| PR | `oidc` | esoi | oidc | upgrade-minor | _(gke)_ | — | `oidc` | `elasticsearch` | — |
| PR | `keycloak-mt` | kemt | keycloak | upgrade-minor | _(gke)_ | — | `keycloak-external` | `elasticsearch` | `[multitenancy]` |
| PR | `keycloak-rba` | kerba | keycloak | upgrade-minor | _(gke)_ | — | `keycloak` | `elasticsearch` | `[rba]` |
| PR | `keycloak-original` | keyc | keycloak | upgrade-minor | _(gke)_ | identity=`keycloak-external`, persistence=`elasticsearch-external` | `keycloak-external` | `elasticsearch-external` | — |
| PR | `elasticsearch` (hybrid) | eshy | hybrid | install | _(gke)_ | — | `hybrid` | `elasticsearch` | — |
| Nightly | `elasticsearch` | eskey | keycloak | _(all)_ | _(gke)_ | — | `keycloak` | `elasticsearch` | — |
| Nightly | `elasticsearch-basic` | esba | basic | _(all)_ | _(gke)_ | — | `basic` | `elasticsearch` | — |
| Nightly | `elasticsearch-oidc` | esoi | oidc | _(all)_ | _(gke)_ | — | `oidc` | `elasticsearch` | — |
| Nightly | `multitenancy` | mtke | keycloak | _(all)_ | _(gke)_ | — | `keycloak-external` | `elasticsearch` | `[multitenancy]` |
| Nightly | `opensearch` (keycloak) | oske | keycloak | _(all)_ | _(gke)_ | — | `keycloak` | `opensearch` | — |
| Nightly | `opensearch` (basic) | osba | basic | _(all)_ | _(gke)_ | — | `basic` | `opensearch` | — |
| Nightly | `identity-migration` | idmike | keycloak | _(all)_ | _(gke)_ | — | `keycloak` | `elasticsearch` | — |
| Nightly | `elasticsearch` (enterprise) | elbae | basic | _(all)_ | _(gke)_ | — | `basic` | `elasticsearch` | — |
| Nightly | `documentstore` | docstr | keycloak | _(all)_ | _(gke)_ | — | `keycloak` | `elasticsearch` | `[documentstore]` |
| Nightly | `keycloak-external-mimic` | kcextm | keycloak | _(all)_ | _(gke)_ | — | `keycloak-external` | `elasticsearch` | — |

Note: `elasticsearch-basic` (PR) and `elasticsearch-oidc` (nightly) are **disabled**.
`opensearch` with keycloak auth (nightly) is **disabled**.

#### Resolved Values Files

**`elasticsearch` (keycloak) on GKE:**
```
values/base.yaml
values/identity/keycloak.yaml
values/persistence/elasticsearch.yaml
values/platform/gke.yaml
```

**`elasticsearch-basic` on GKE:**
```
values/base.yaml
values/identity/basic.yaml
values/persistence/elasticsearch.yaml
values/platform/gke.yaml
```

**`elasticsearch` (hybrid) on GKE:**
```
values/base.yaml
values/identity/hybrid.yaml
values/persistence/elasticsearch.yaml
values/platform/gke.yaml
```

**`oidc` on GKE:**
```
values/base.yaml
values/identity/oidc.yaml
values/persistence/elasticsearch.yaml
values/platform/gke.yaml
```

**`keycloak-mt` on GKE:**
```
values/base.yaml
values/identity/keycloak-external.yaml
values/persistence/elasticsearch.yaml
values/platform/gke.yaml
values/features/multitenancy.yaml
```

**`keycloak-rba` on GKE:**
```
values/base.yaml
values/identity/keycloak.yaml
values/persistence/elasticsearch.yaml
values/platform/gke.yaml
values/features/rba.yaml
```

**`keycloak-original` on GKE:**
```
values/base.yaml
values/identity/keycloak-external.yaml
values/persistence/elasticsearch-external.yaml
values/platform/gke.yaml
```

**`opensearch` (basic) on GKE:**
```
values/base.yaml
values/identity/basic.yaml
values/persistence/opensearch.yaml
values/platform/gke.yaml
```

**`keycloak-external-mimic` on GKE:**
```
values/base.yaml
values/identity/keycloak-external.yaml
values/persistence/elasticsearch.yaml
values/platform/gke.yaml
```

**`upgrade-migration` on GKE (flow=upgrade-minor):**
```
values/base.yaml
values/identity/keycloak.yaml
values/persistence/elasticsearch.yaml
values/platform/gke.yaml
```
(Upgrade flag is set from flow containing "upgrade", which would include
`base-upgrade.yaml` if `Upgrade == true` in the config.)

**`documentstore` on GKE:**
```
values/base.yaml
values/identity/keycloak.yaml
values/persistence/elasticsearch.yaml
values/platform/gke.yaml
values/features/documentstore.yaml
```

#### What 8.8 Adds Over 8.7
- `identity/basic.yaml` — No IdP, basic auth only
- `identity/hybrid.yaml` — Combined Identity + OIDC orchestration auth
- Unit tests restructured: `Management`, `Orchestration`, `Design` (no more
  separate Zeebe/Operate/Tasklist)

---

### 8.9

Adds RDBMS persistence backends and a second `keycloak-original` entry.

#### Scenarios

| Tier | Scenario | Shortname | Auth | Flow | Platforms | Explicit Fields | Resolved Identity | Resolved Persistence | Resolved Features |
|------|----------|-----------|------|------|-----------|-----------------|-------------------|----------------------|-------------------|
| PR | `elasticsearch` | eske | keycloak | _(all)_ | gke, eks | — | `keycloak` | `elasticsearch` | — |
| PR | `opensearch` | oske | keycloak | _(all)_ | _(gke)_ | — | `keycloak` | `opensearch` | — |
| PR | `upgrade-migration` | eske-upgm | keycloak | upgrade-minor | _(gke)_ | — | `keycloak` | `elasticsearch` | — |
| PR | `elasticsearch-basic` | esba | basic | install | _(gke)_ | — | `basic` | `elasticsearch` | — |
| PR | `oidc` | esoi | oidc | upgrade-minor | _(gke)_ | — | `oidc` | `elasticsearch` | — |
| PR | `keycloak-mt` | kemt | keycloak | upgrade-minor | _(gke)_ | — | `keycloak-external` | `elasticsearch` | `[multitenancy]` |
| PR | `keycloak-rba` | kerba | keycloak | upgrade-minor | _(gke)_ | — | `keycloak` | `elasticsearch` | `[rba]` |
| PR | `keycloak-original` | keyc | keycloak | upgrade-minor | _(gke)_ | identity=`keycloak-external`, persistence=`elasticsearch-external` | `keycloak-external` | `elasticsearch-external` | — |
| PR | `keycloak-original` | keorg | keycloak | install | _(gke)_ | identity=`keycloak-external`, persistence=`elasticsearch-external` | `keycloak-external` | `elasticsearch-external` | — |
| PR | `gateway-keycloak` | gatkc | keycloak | install | _(gke)_ | — | `keycloak` | `elasticsearch` | — |
| Nightly | `elasticsearch` | eskey | keycloak | _(all)_ | _(gke)_ | — | `keycloak` | `elasticsearch` | — |
| Nightly | `elasticsearch-basic` | esba | basic | _(all)_ | _(gke)_ | — | `basic` | `elasticsearch` | — |
| Nightly | `elasticsearch-oidc` | esoi | oidc | _(all)_ | _(gke)_ | — | `oidc` | `elasticsearch` | — |
| Nightly | `multitenancy` | mtke | keycloak | _(all)_ | _(gke)_ | — | `keycloak-external` | `elasticsearch` | `[multitenancy]` |
| Nightly | `opensearch` (keycloak) | oske | keycloak | _(all)_ | _(gke)_ | — | `keycloak` | `opensearch` | — |
| Nightly | `opensearch` (basic) | osba | basic | _(all)_ | _(gke)_ | — | `basic` | `opensearch` | — |
| Nightly | `identity-migration` | idmike | keycloak | _(all)_ | _(gke)_ | — | `keycloak` | `elasticsearch` | — |
| Nightly | `elasticsearch` (enterprise) | elbae | basic | _(all)_ | _(gke)_ | — | `basic` | `elasticsearch` | — |
| Nightly | `documentstore` | docstr | keycloak | _(all)_ | _(gke)_ | — | `keycloak` | `elasticsearch` | `[documentstore]` |
| Nightly | `keycloak-external-mimic` | kcextm | keycloak | _(all)_ | _(gke)_ | — | `keycloak-external` | `elasticsearch` | — |

Note: `upgrade-migration`, `elasticsearch-basic` (PR), `elasticsearch-oidc`, and
`opensearch` with keycloak (nightly) are **disabled**.

#### Resolved Values Files

All resolutions from 8.8 apply identically, plus:

**`opensearch` (keycloak, PR) on GKE:**
```
values/base.yaml
values/identity/keycloak.yaml
values/persistence/opensearch.yaml
values/platform/gke.yaml
```

**`keycloak-original` (keorg, flow=install) on GKE:**
```
values/base.yaml
values/identity/keycloak-external.yaml
values/persistence/elasticsearch-external.yaml
values/platform/gke.yaml
```
(Same files as the `keyc` entry — only the flow and shortname differ.)

**`gateway-keycloak` on GKE:**
```
values/base.yaml
values/identity/keycloak.yaml
values/persistence/elasticsearch.yaml
values/platform/gke.yaml
```

#### What 8.9 Adds Over 8.8
- `persistence/rdbms.yaml` — PostgreSQL RDBMS secondary storage
- `persistence/rdbms-oracle.yaml` — Oracle RDBMS secondary storage
- Second `keycloak-original` entry (`keorg`) with `flow: install`
- `opensearch` enabled in PR tier
- `gateway-keycloak` scenario
- Documentation comments in ci-test-config.yaml describing the Selection +
  Composition approach

---

## Known Discrepancies

The following scenarios rely on name parsing and have `auth` fields that do not
align with the derived identity. These work correctly today because the layered
path ignores the `auth` field (it uses `identity` instead), but adding explicit
`identity` fields would improve clarity:

| Version | Scenario | `auth` field | Derived Identity | Notes |
|---------|----------|--------------|------------------|-------|
| 8.7–8.9 | `keycloak-mt` | keycloak | `keycloak-external` | Name parsing maps `-mt` to external keycloak |
| 8.6–8.9 | `multitenancy` | keycloak | `keycloak-external` | Name parsing maps `multitenancy` to external keycloak |
| 8.8–8.9 | `keycloak-external-mimic` | keycloak | `keycloak-external` | Name parsing maps `keycloak-external` substring |
| 8.8–8.9 | `elasticsearch` (hybrid) | hybrid | `hybrid` | `auth` and derived identity happen to align |
| 8.8–8.9 | `elasticsearch-basic` | basic | `basic` | `auth` and derived identity happen to align |

The `auth` field is still used by the **legacy Taskfile path** for scenarios on
versions without layered values (8.2–8.5). For 8.6+, the `auth` field is
effectively ignored in the layered code path.
