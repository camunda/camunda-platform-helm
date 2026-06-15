---
title: CI Scenario Matrix and Index-Prefix Coverage
---

# CI Scenario Matrix and Index-Prefix Coverage

## Feature matrix

One row per `(chart version x platform x scenario name x shortname)` exercised by a cron-triggered nightly workflow under `camunda/c8-cross-component-e2e-tests/.github/workflows/playwright_sm_nightly_*.yml`.

- Scenarios live in the composable registry at `charts/camunda-platform-<v>/test/ci/registry/`: `manifest.yaml` lists each scenario (`id`, `shortname`, `tier`, `enabled`), and `scenarios/<id>.yaml` carries its `name`, `flows`, `identity`, `persistence`, `features`, `platforms`, and optional `prefix-key`.
- **PR CI** is `enabled` when the scenario's `manifest.yaml` entry has `enabled: true`.
- **Prefix-aligned** is `OK` for upgrade pairs covered by `TestUpgradePrefixCoverage`; install-only rows are `n/a`.
- Each upgrade is split into two registry scenarios sharing one logical `name`/`shortname`: an `*-upg-install` leg (`flow: install`, the previous chart version's install) and an `*-upg-modular` leg (`flow: modular-upgrade-minor`). Legs whose logical index prefix must match across the two charts carry `prefix-key`.
- This table is hand-maintained documentation that mirrors the external nightly repo. The gap-enablement and the upgrade prefix invariant below are the parts enforced by tests; the workflow-to-row mapping itself is not test-enforced (it depends on an external repo).

| version | platform | scenario | shortname | flow | PR CI | nightly workflow | prefix-aligned |
|---|---|---|---|---|---|---|---|
| 8.7 | eks, gke | qa-elasticsearch | qael | install | disabled | `playwright_sm_nightly_tests_chrome_8_7.yml`, `playwright_sm_nightly_tests_edge_8_7.yml`, `playwright_sm_nightly_tests_firefox_8_7.yml` | n/a |
| 8.7 | gke | qa-elasticsearch-mt | qamt | install | disabled | `playwright_sm_nightly_mt_8_7.yml` | n/a |
| 8.7 | gke | qa-elasticsearch-rba | qarba | install | disabled | `playwright_sm_nightly_rba_8_7.yml` | n/a |
| 8.7 | gke | qa-opensearch | qaos | install | disabled | `playwright_sm_nightly_tests_opensearch_8_7.yml` | n/a |
| 8.8 | eks, gke | qa-document-store | qadoc | install | disabled | `playwright_sm_nightly_document_store_8_8.yml` | n/a |
| 8.8 | gke | qa-elasticsearch-mt | qaelmt | install | disabled | `playwright_sm_nightly_mt_8_8.yml` | n/a |
| 8.8 | gke | qa-elasticsearch-mt-upg | qaelmtup2 | install, modular-upgrade-minor | disabled | `playwright_sm_nightly_upgrade_minor_mt_8_9.yml` (`8-8-install`) | OK |
| 8.8 | gke | qa-elasticsearch-upg | qaelupg | install, modular-upgrade-minor | disabled | `playwright_sm_nightly_tests_chrome_8_8.yml`, `playwright_sm_nightly_upgrade_minor_8_8.yml` (`8-8-plus-upgrade`) | OK |
| 8.8 | gke | qa-opensearch-upg | qaosupg2 | install | disabled | `playwright_sm_nightly_tests_opensearch_8_8.yml` | n/a |
| 8.9 | eks, gke | qa-document-store | qadoc | install | disabled | `playwright_sm_nightly_document_store_8_9.yml` | n/a |
| 8.9 | eks, gke | qa-document-store-upg | qadocupg | modular-upgrade-minor | **enabled** | `playwright_sm_nightly_upgrade_minor_document_store_8_9.yml` (`8-9-upgrade`) | OK |
| 8.9 | gke | qa-elasticsearch-mt-upg | qaelmtupg | modular-upgrade-minor | disabled | `playwright_sm_nightly_upgrade_minor_mt_8_9.yml` (`8-9-upgrade`) | OK |
| 8.9 | gke | qa-elasticsearch-upg | qaelupg | install, modular-upgrade-minor | disabled | `playwright_sm_nightly_tests_chrome_8_9.yml`, `playwright_sm_nightly_upgrade_minor_8_9.yml` (`8-9-upgrade`) | OK |
| 8.9 | gke | qa-opensearch-upg | qaosupg | modular-upgrade-minor | **enabled** | `playwright_sm_nightly_upgrade_minor_opensearch_8_9.yml` (`8-9-upgrade`) | OK |
| 8.10 | eks, gke | qa-document-store | qadoc | install | disabled | `playwright_sm_nightly_document_store_8_10.yml` | n/a |
| 8.10 | eks, gke | qa-document-store-upg | qadocupg | modular-upgrade-minor | **enabled** | `playwright_sm_nightly_upgrade_minor_document_store_8_10.yml` (`8-10-upgrade`) | OK |
| 8.10 | gke | qa-elasticsearch-mt | qaelmt | install | disabled | `playwright_sm_nightly_mt_8_10.yml` | n/a |
| 8.10 | gke | qa-elasticsearch-mt-upg | qaelmtupg | modular-upgrade-minor | **enabled** | `playwright_sm_nightly_upgrade_minor_mt_8_10.yml` (`8-10-upgrade`) | OK |
| 8.10 | gke | qa-elasticsearch-rba | qaelrba | install | disabled | `playwright_sm_nightly_rba_8_10.yml` | n/a |
| 8.10 | gke | qa-elasticsearch-upg | qaelupg | install, modular-upgrade-minor | disabled | `playwright_sm_nightly_tests_chrome_8_10.yml`, `playwright_sm_nightly_upgrade_minor_8_10.yml` (`8-10-upgrade`) | OK |
| 8.10 | gke | qa-opensearch | qaos | install | disabled | `playwright_sm_nightly_tests_opensearch_8_10.yml` | n/a |
| 8.10 | gke | qa-opensearch-upg | qaosupg | modular-upgrade-minor | **enabled** | `playwright_sm_nightly_upgrade_minor_opensearch_8_10.yml` (`8-10-upgrade`) | OK |

The five **enabled** modular-upgrade-minor rows above are the issue #6172 coverage gaps that this change turned on; `TestIssue6172UpgradeGapsRunInPRCI` asserts each is generated into the matrix. Tier-1/tier-2 PR rows not exercised by cron nightlies are omitted; see each version's `ci/registry/manifest.yaml` for the full PR matrix. Some `*-upg-install` and tasklist-v1 install legs are also registered (disabled by default) — run `deploy-camunda matrix list --include-disabled --versions <v>` for the exhaustive list.

## Index-prefix coverage

Per `(version x persistence layer)`, which `$*_INDEX_PREFIX` placeholders appear in `charts/camunda-platform-<v>/test/integration/scenarios/chart-full-setup/values/persistence/<L>.yaml`. `Y` means referenced, `-` means the file exists but has no reference, and `n/a` means the file does not exist for this version.

### 8.7

| persistence | ORCH | OPERATE | TASKLIST | OPTIMIZE |
|---|---|---|---|---|
| elasticsearch | - | - | - | - |
| elasticsearch-external | Y | Y | Y | Y |
| elasticsearch-self-signed | - | - | - | - |
| opensearch | Y | Y | Y | Y |
| opensearch-embedded | Y | Y | Y | Y |
| opensearch-external | Y | Y | Y | Y |
| no-elasticsearch, rdbms*, elasticsearch-external-self-signed | n/a | n/a | n/a | n/a |

### 8.8

| persistence | ORCH | OPERATE | TASKLIST | OPTIMIZE |
|---|---|---|---|---|
| elasticsearch | - | - | - | - |
| elasticsearch-external | Y | - | - | Y |
| elasticsearch-self-signed | - | - | - | - |
| elasticsearch-external-self-signed | Y | - | - | Y |
| opensearch | Y | - | - | Y |
| opensearch-embedded | Y | Y | - | Y |
| opensearch-external | Y | - | - | Y |
| no-elasticsearch, rdbms* | n/a | n/a | n/a | n/a |

### 8.9, 8.10

| persistence | ORCH | OPERATE | TASKLIST | OPTIMIZE |
|---|---|---|---|---|
| elasticsearch, elasticsearch-self-signed, no-elasticsearch, rdbms* | - | - | - | - |
| elasticsearch-external | Y | - | - | Y |
| opensearch | Y | - | - | Y |
| opensearch-embedded | Y | Y | - | Y |
| opensearch-external | Y | - | - | Y |
| elasticsearch-external-self-signed | n/a | n/a | n/a | n/a |

### Observations

- `TASKLIST_INDEX_PREFIX` is consumed only on 8.7 for Tasklist's own indices. Zeebe import indices use `ORCHESTRATION_INDEX_PREFIX`.
- `OPERATE_INDEX_PREFIX` is consumed only by `opensearch-embedded.yaml` in 8.8/8.9/8.10. It disambiguates `CAMUNDA_DATA_SECONDARYSTORAGE_OPENSEARCH_INDEXPREFIX` from `global.opensearch.prefix`, which uses `ORCHESTRATION_INDEX_PREFIX`.
- `ORCHESTRATION_INDEX_PREFIX` and `OPTIMIZE_INDEX_PREFIX` are consumed by every Elasticsearch/OpenSearch persistence layer that overrides defaults. Bundled `elasticsearch.yaml` and `elasticsearch-self-signed.yaml` rely on chart defaults and reference no prefix variables.

The upgrade prefix invariant (install-step `$*_INDEX_PREFIX` placeholders ⊆ upgrade-step placeholders) and the issue #6172 gap enablement are enforced by [`upgrade_prefix_coverage_test.go`](../scripts/deploy-camunda/matrix/upgrade_prefix_coverage_test.go) — `TestUpgradePrefixCoverage` and `TestIssue6172UpgradeGapsRunInPRCI`. Both read enabled scenarios from the generated matrix (`matrix.Generate`) rather than parsing config directly, so they track the composable registry. Where an upgrade's install leg runs on a previous chart version that lacks one of the current version's layers (e.g. the `postgresql-companion` feature on 8.10 has no counterpart on 8.9), the prefix check skips that pair rather than failing — prefix alignment, not cross-version layer parity, is what it asserts.

## Maintenance

When the registry, persistence YAML, or a cross-component cron nightly changes:

- Update the feature matrix above to reflect the new or removed row.
- After editing any `manifest.yaml` / `scenarios/*.yaml`, regenerate the golden with `make go.update-registry-golden` and commit the updated `charts/<v>/test/ci/registry-snapshot.yaml`.
- Keep `prefix-key` on upgrade legs whose install leg uses a different scenario name, so both charts generate the same logical index prefixes.

Sources of truth:

| Concern | Source |
|---|---|
| Which scenarios run on PR CI per version | `charts/camunda-platform-<v>/test/ci/registry/manifest.yaml` (`integration.scenarios[].enabled`) + `registry/scenarios/<id>.yaml` |
| Which nightly E2E demands a `(version, platform, scenario, shortname, flow)` | `camunda/c8-cross-component-e2e-tests` repo, `.github/workflows/playwright_sm_nightly_*.yml` |
| Which prefix variables a persistence layer references | `charts/camunda-platform-<v>/test/integration/scenarios/chart-full-setup/values/persistence/<L>.yaml` |
| How prefix env-vars are generated | `scripts/deploy-camunda/deploy/scenario.go` `generateScenarioContext()` and `scripts/deploy-camunda/deploy/values.go` `buildScenarioEnv()` |

See also: [Integration Test Scenario Resolution](./skills/integration-test-scenario-resolution.md), [GitHub Actions Workflows](./reference/github-actions-workflows.md).
