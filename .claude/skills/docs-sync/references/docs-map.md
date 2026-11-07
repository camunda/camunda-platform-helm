# Where chart changes land in camunda-docs

Paths below are relative to a version root: `docs/` for Next, or
`versioned_docs/version-<minor>/` for a released minor. A page can be missing or
named differently in older versions, so confirm each path exists in every target
folder before editing (the docs repo calls this "doc ID divergence").

## Finding pages: search first, then use this map

The most reliable way to find affected pages is to search the target version
folder for the exact values key and for its parent key:

```bash
grep -rn --include='*.md' -e 'orchestration.exporters.zeebe' <version-root>/self-managed
grep -rn --include='*.md' -e 'exporters:' <version-root>/self-managed   # YAML-snippet form
```

Docs often show values as YAML blocks rather than dotted keys, so search for the
leaf key inside YAML too. Use the map below to decide where *new* content
belongs when nothing mentions the key yet.

| Chart area (values prefix or template dir) | Primary page(s) under `self-managed/deployment/helm/` |
|---|---|
| Top-level sections, new values files (`values-*.yaml`) | `chart-parameters.md` |
| `global.topology.*`, split releases, Hub/Optimize releases | `install/topology/*.md`, `configure/configure-physical-tenants.md` |
| `global.identity.*`, `identity.*`, auth, OIDC, Keycloak | `configure/authentication-and-authorization/*.md` |
| `global.ingress.*`, `*.ingress.*`, Gateway API, nginx annotations | `configure/ingress/*.md` |
| `global.tls.*`, `*.tls.*`, `caBundle` | `configure/tls.md`, `configure/orchestration-tls-modes.md` |
| `global.elasticsearch|opensearch.*`, `*.database.*`, RDBMS | `configure/database/**` |
| `optimize.database.*` | `configure/database/optimize/*.md` |
| `global.image.*`, `*.image.*`, registries, pull secrets | `configure/registry-and-images/*.md` |
| `global.license.*` | `configure/license-key.md` |
| `*.secret.*`, `existingSecret`, `inlineSecret` patterns | `configure/secret-management.md` |
| `*.service.*`, `appProtocol`, ports | `configure/service-configuration.md` |
| `dnsPolicy`, `dnsConfig`, `hostNetwork` | `configure/pod-networking.md` |
| `nodeSelector`, `affinity`, `tolerations`, topology spread | `configure/pod-scheduling.md` |
| Probes (`startupProbe`, `readinessProbe`, `livenessProbe`) | `configure/health-probes.md` |
| `configuration`, `extraConfiguration`, `overrideConfiguration` | `configure/application-configs.md`, `configure/configuration-responsibilities.md` |
| `extraManifests` | `configure/add-extra-manifests.md` |
| Zone awareness, `orchestration.partitioning`, multi-region | `configure/multi-region-zone-awareness.md`, `operational-tasks/zone-aware-migration.md` |
| Enabling optional components (Optimize, Connectors, Hub) | `configure/enable-additional-components.md` |
| Custom connectors | `configure/running-custom-connectors.md` |
| Data retention / ILM | `configure/data-retention.md` |
| Helm CLI v3/v4 requirements | `operational-tasks/helm-v4.md`, `operational-tasks/moving-helm-v3-to-v4.md` |
| Schema validation, `global.strictValues`, unknown keys | `operational-tasks/helm-values-schema-validation.md` |
| Removed Bitnami subcharts and their keys | `operational-tasks/migration-from-bitnami/index.md` |
| Install defaults, quick install | `install/quick-install.md`, `install/production/index.md` |

## Upgrade guides

Helm upgrade guides live in `self-managed/upgrade/helm/<from>-to-<to>.md`
(for example `890-to-8100.md` is the 8.9 → 8.10 guide). They have an
"Update your values file to <minor>" section with one `###` subsection per
change. Removed, renamed, and default-changed keys that need user action
belong there, with a before/after YAML snippet.

If the guide for the target minor does not exist yet (common for Next), do not
create a new page on your own: adding pages needs sidebar and redirect work.
Put the change in the release announcements instead, and say in the plan that
the upgrade guide is missing so the engineer can decide.

## Release announcements and release notes

Each minor has three files under
`reference/announcements-release-notes/<minor-without-dot>0/`
(8.11 → `8110/`, 8.10 → `8100/`):

| File | Gets an entry when |
|---|---|
| `<v>-announcements.md` | The change breaks existing behavior or requires action before upgrading: removed or renamed keys, default changes that alter behavior, new hard failures, newly deprecated keys with a removal target. |
| `<v>-release-notes.md` | A user-visible feature or enhancement ships in a release tag. Add it under the section for the tag where it first appears (`## 8.11.0-alpha2`, and so on). If the tag section does not exist yet, ask the engineer which tag the chart release maps to. |
| `whats-new-in-<v>.md` | Never automatically. It is curated; mention it in the plan only for large features. |

Announcement entries use the badge layout already in the file (copy the nearest
existing Helm or Self-Managed entry, including `{#anchor}`). Put the entry under
the existing Self-Managed or deployment heading; read the file's headings first.

The release-notes folder exists both in `docs/` and in each `versioned_docs/`
copy. Edit the copy inside every target version root the change maps to, the
same as other pages.

Changes to a minor that is already released (a patch chart release) rarely
need a release-notes entry, because those files are organized by alpha and GA
tags. Breaking or action-required changes in a patch release still go in the
announcements for that minor.

## What does not need docs

- Image tag or digest bumps (`image_bumps` in the script output).
- Test, CI, golden-file, and `values-digest.yaml` changes.
- Description-only edits in `values.yaml` that fix typos or wording. Description
  edits that change meaning (new constraint, new supported value, new
  deprecation) do count.
- Internal refactors of templates that render identical manifests. If the PR
  touches templates, read the PR description and the golden-file diff in
  `test/` to tell whether rendered output changed.
