# Operational Skills

Operational runbooks live as on-demand skills under `.claude/skills/<name>/SKILL.md`
(Agent Skills format — plain markdown, readable by any agent; Claude Code loads them
automatically on trigger). This file is the index and is kept at this path because
external tooling and older references point here.

Throughout the skills, `$NS` refers to a Kubernetes namespace and `$RELEASE` to a Helm release name.

| Skill | Covers |
|---|---|
| [`deploy-camunda`](.claude/skills/deploy-camunda/SKILL.md) | CLI install and pre-flight, single-scenario and composed deploys, configuration profiles, matrix operations, SNAPSHOT image tags, render-without-deploy, `prepare-values`, `watch`, values troubleshooting |
| [`gke-verification`](.claude/skills/gke-verification/SKILL.md) | End-to-end fix verification on GKE: pre-flight checklist, deploy the exact CI scenario with `watch`, generate credentials, reproduce on `main` then verify on the branch, clean up |
| [`rfr-validation`](.claude/skills/rfr-validation/SKILL.md) | PR Ready-for-Review validation: tier-1/tier-2 reference and shortname decoder, scenario selection by diff, tier-2 verification before merge, RFR checklist, optional `crev` self-check, anti-patterns |
| [`cluster-debugging`](.claude/skills/cluster-debugging/SKILL.md) | kubectl triage of failing pods, pod naming, port-forwarding, secrets, namespace lifecycle, Spring Boot `/actuator/configprops` inspection, headless JVM debugging with `jdb` |
| [`e2e-testing`](.claude/skills/e2e-testing/SKILL.md) | Generating per-environment `.env` files, running Playwright suites via `c8e2e` or `--test-e2e`, parallel environments, reproducing CI e2e failures locally |
| [`ci-scenario-authoring`](.claude/skills/ci-scenario-authoring/SKILL.md) | New persistence layers and CI scenarios, features/shortnames, pre-install / post-deploy / pre-upgrade lifecycle hooks, `TestLifecycleFixtures` contract |
| [`helm-values-debugging`](.claude/skills/helm-values-debugging/SKILL.md) | Helm array-replace merge vs `deploy-camunda` name-keyed merge, neutralizing parent-chart defaults, Bitnami env-var chains |

Related step-by-step guides under `docs/skills/`:

- [`docs/skills/reproducing-ci-e2e-failures.md`](docs/skills/reproducing-ci-e2e-failures.md) — pull CI logs/artifacts, decode shortnames, spin up an identical local environment
- [`docs/skills/integration-test-scenario-resolution.md`](docs/skills/integration-test-scenario-resolution.md) — how `deploy-camunda` resolves layered values per chart version
