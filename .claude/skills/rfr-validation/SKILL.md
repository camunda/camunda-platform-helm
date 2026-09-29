---
name: rfr-validation
description: Validate a PR locally before marking it Ready-for-Review — map the diff to CI tier-1/tier-2 scenarios, decode shortnames, run the minimum correct scenario set via deploy-camunda matrix run, verify tier-2 scenarios before merge, complete the RFR checklist, optionally self-check with crev. Use when preparing to mark a PR ready, asked which scenarios to run for a change, or validating tier-2 scenario changes the merge queue would otherwise catch.
---

# PR Ready-for-Review Validation

PR CI runs **tier-1 only** (7 deploys, the `eske` baseline). The full matrix (81 deploys) runs in the **merge queue** and rejects any PR whose diff exercises a non-baseline variant (OpenSearch, RDBMS, auth, document store, hub, TLS, ARM Elasticsearch, no-secondary-storage) and fails. Run the minimum correct scenario set locally before marking the PR Ready-for-Review.

## Prerequisites

- `deploy-camunda` — `make install.dx-tooling`
- `helm`, `kubectl` — `asdf install` (see `.tool-versions`)
- `gh` — PR and workflow-run inspection
- `crev` — automated PR reviewer at [github.com/camunda/crev](https://github.com/camunda/crev); see `docs/contribution-and-collaboration.md` and `.github/escalation-policy.md` for the project workflow
- `actionlint` — `brew install actionlint` (macOS) or `go install github.com/rhysd/actionlint/cmd/actionlint@latest`

## Identify the Surface

1. **Chart versions touched** — which `charts/camunda-platform-<v>/`.
2. **Variant axes touched** — `persistence` (ES/OS/RDBMS), `auth` (keycloak/basic/none/orgs/multitenancy), `feature` (docstr, huble, migrator), `infra` (arm, platform).
3. **Whether `eske` covers it** — `eske` = elasticsearch + keycloak on GKE. If yes, tier-1 alone is enough.

## Tier Reference

Authoritative source: `tier:` and `enabled:` live in the composable registry `charts/camunda-platform-<v>/test/ci/registry/manifest.yaml` (8.6 predates the registry and has no active CI). Per-version shortname lists are deliberately **not** duplicated here — they go stale. Derive them:

```bash
deploy-camunda matrix list --tier 1                 # the PR gate, every version
deploy-camunda matrix list --tier 2 --versions <v>  # merge-queue variants for one version
deploy-camunda matrix list --versions <v>           # everything the merge queue runs
```

A scenario that declares no `tier:` matches neither filter and is reachable only through the unfiltered matrix, so `--tier 1` plus `--tier 2` equals the full set only while every enabled scenario carries an explicit tier.

**Tier 1:** `eske` on every version, plus `keyco` on 8.7 and 8.10. 8.9's `eske` covers both `install` and `upgrade-minor`; every other tier-1 entry is `install` on GKE.

**Variant decoder:**

| Shortname | Meaning |
|---|---|
| `eske` | Elasticsearch + Keycloak on GKE — the baseline |
| `oske` | OpenSearch (`osem` on 8.7, which predates the rename) |
| `osss` / `osot` | OpenSearch with a self-signed cert; `-os-trust` variant trusts the OS-level bundle |
| `esot` | Elasticsearch self-signed with OS-level trust (8.8, 8.9) |
| `estls` / `estlr` / `estlc` | orchestration TLS; `-rest` and `-rest-appconfig` variants |
| `cotls` / `optls` | Connectors TLS, Optimize TLS |
| `esoi` | Elasticsearch OIDC (Microsoft Entra) |
| `esa0` | Auth0 as the external IdP (8.10) |
| `rdbms` / `rdbss` / `rdbme` | RDBMS persistence; self-signed and external variants |
| `ke*` | Keycloak variants: `kemt` `keycloak-mt`, `kerba` `keycloak-rba`, `keorg`/`keyco`/`keyc` `keycloak-original` |
| `gatkc` | gateway + Keycloak auth path |
| `nosec` | `noSecondaryStorage` (no Elasticsearch; `persistence: no-elasticsearch`, still uses Keycloak auth) |
| `esarm` | ARM Elasticsearch |
| `docstr` | document store feature |
| `cprst` / `cprsu` | component-persistence, install and upgrade |
| `secfl` | file-backed secret store |
| `hub*` | Camunda Hub shapes: `huble`/`hublu` legacy, `hubmu` mixed, `hubwo` web-modeler-only, `hubco` console-only, `hubde` double-enable, `hubpi` pinned images, `hubdb` external DB |
| `mns` / `mns2` / `mnop` / `ptnt` | multi-namespace topologies and physical tenants |
| `shde2` | shadow E2E |
| `entv` | enterprise values overlay (`values-enterprise.yaml`) |

## Select Scenarios

Default: tier-1 on every affected version. Add tier-2 entries only when the diff exercises that variant.

| Diff | Scenarios |
|---|---|
| `templates/orchestration/operate/` on 8.10 | `eske` 8.10 |
| OpenSearch values rework on 8.9 + 8.10 | `eske` + `osem` per version |
| RDBMS migrator change on 8.10 | `eske` + `rdbms` 8.10 |
| Keycloak helper change 8.9–8.10 | `eske` + each version's `ke*` set |
| Document store feature 8.8+ | `eske` + `docstr` per version |
| Hub change on 8.10 | `eske` + `huble` |
| `_helpers.tpl` change | tier-1 all versions + `nosec`, `docstr` |
| `values-enterprise.yaml` or enterprise image tags | `entv` — defined and enabled on 8.7 and 8.9 only; 8.8 defines it but leaves it `enabled: false`, and 8.10 does not define it at all |

**Skip the matrix** for `.github/workflows/*` (run `actionlint`), `scripts/` Go tooling (`make go.test`), Dockerfile-only (`hadolint`, `docker build --target`), compose-only (`docker compose config`), docs-only.

## Run via deploy-camunda

CI uses `deploy-camunda matrix run` (Taskfile orchestration removed in PR #6016). Rebuild the binary after every pull — see the `deploy-camunda` skill.

```bash
# PR-CI baseline
deploy-camunda matrix list --tier 1 --versions 8.10

# Tier-2 merge-queue set
deploy-camunda matrix list --tier 2 --versions 8.10

# Full merge-queue set (tier-2 plus untiered)
deploy-camunda matrix list --versions 8.10

# Run one scenario
deploy-camunda matrix run \
  --versions 8.10 --shortname-filter eske --shortname-exact \
  --flow-filter install --platform gke \
  --ingress-base-domain-gke ci.distro.ultrawombat.com

# CI-parity overrides: --extra-helm-arg, --extra-helm-set
```

GKE matrix runs require `--ingress-base-domain-gke` — the host is computed per-namespace from that base domain; without it, `${CAMUNDA_HOSTNAME}` substitution in `base.yaml` fails with `missing required environment variables: CAMUNDA_HOSTNAME`.

## Verifying tier-2 scenarios before merge

**When:** your PR adds or changes a tier-2 scenario (or a diff exercises one). PR CI runs tier-1 only; the merge queue runs tier-2. Validate locally before merge to avoid a slow, expensive red merge queue.

**The loop:**

1. `deploy-camunda matrix list --tier 2 --versions <v>` — confirm your scenarios, tier, and `enabled` status.
2. `deploy-camunda matrix run … --dry-run` — offline gate: layers resolve, exactly the expected entries, no template errors.
3. Real run (comma-separated `--shortname-filter` runs several in one invocation; per-entry PASS/FAIL summary at the end):

```bash
deploy-camunda matrix run \
  --versions 8.10 --shortname-filter <sn1>,<sn2> --shortname-exact \
  --flow-filter upgrade-minor --platform gke \
  --ingress-base-domain-gke ci.distro.ultrawombat.com \
  --delete-namespace --timeout 25 --yes
```

4. Fix failures, then re-run just the failing shortname. Upgrade flows re-install the prior minor each run (~10–15 min/scenario).

**Credentials:** export (or place in `.env`) before running:

- `TEST_DOCKER_USERNAME_CAMUNDA_CLOUD` / `TEST_DOCKER_PASSWORD_CAMUNDA_CLOUD` — Harbor pull secret.
- For `postgresql-companion` scenarios: `RDBMS_POSTGRESQL_USERNAME` / `RDBMS_POSTGRESQL_PASSWORD`.

Obtain values from your team's secret store. `deploy-camunda doctor` and `matrix run --dry-run` validate presence per entry.

**Local-run gotchas:**

- **`--ingress-base-domain-gke` required** on GKE — else `CAMUNDA_HOSTNAME` substitution fails.
- **`post-infra` migration hooks and asdf:** hooks like `post-infra-bitnami-migration` clone `camunda-deployment-references` and run under *its* `.tool-versions`; an uninstalled pinned tool (e.g. `jq 1.7.1`) makes the asdf shim exit 126. Fix: `asdf install <tool> <version>` for the versions that clone pins. This is a local-env issue, not a chart bug.
- A tier-2 failure that is clearly local-environment (exit 126, missing creds) is **not** a merge-queue signal — fix the environment and re-run.

**Flow semantics** (`upgrade-minor` two-step vs `modular-upgrade-minor` single-step): see the `deploy-camunda` skill.

## RFR Checklist

- [ ] Affected chart versions enumerated.
- [ ] Tier-1 (`eske`) passes on each affected version.
- [ ] Each diff-implied tier-2 scenario passes.
- [ ] `make go.update-golden-only chartPath=...` executed if templates changed, and updated goldens committed.
- [ ] `make precommit.chores` clean.
- [ ] Optional: `crev` dry-run produces no findings (see below).

## Optional Pre-RFR Self-Check (`crev`)

`crev` ([github.com/camunda/crev](https://github.com/camunda/crev)) runs automatically on every PR per `docs/contribution-and-collaboration.md` and `.github/escalation-policy.md`. After review it posts a `crev/escalation` commit status plus a `human-review-required` or `ai-review-sufficient` label.

Running `crev` locally before flipping draft → Ready-for-Review is optional, not required, and can surface findings before reviewers see them:

```bash
crev <pr-url> --single --dry-run
```

`crev` defaults to dry-run and does not post comments. A typical run takes 1-5 minutes. Output terminates in a JSON object (`schema: "crev/v1"`, `findings: [...]`, `summary: "..."`). `findings.length == 0` is the green signal.

- Multi-PR sibling discovery is enabled by default; use `--single` for unrelated PRs.
- The cache key includes head SHAs and reviewer configuration, so pushing new commits invalidates the cache automatically.
- Matrix scenarios prove chart correctness; `crev` is the domain-aware review pass and the primary automated signal for non-chart PRs.

## Anti-Patterns

- Treating PR CI green as sufficient when the diff exercises a non-baseline variant; the merge queue will reject the PR.
- Hand-editing golden files instead of running `make go.update-golden-only`.
- Adding speculative tier-2 entries not exercised by the diff (consumes local capacity without coverage gain).
- Reading the tier list above without re-checking via `deploy-camunda matrix list --tier 2` (or the version-appropriate YAML/registry source).
- Using the merge queue as a discovery mechanism for predictable variant breakage.
- Running the matrix on PRs that change no rendering output (workflow, Dockerfile, compose, docs, Go-tooling).
