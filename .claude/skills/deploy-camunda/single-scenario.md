# Deploy one scenario with today's CLI

A step-by-step runbook for a single-scenario deploy with the root `deploy-camunda` command
(no `matrix run`). Every flag and behaviour here is checked against `main` and cited by file;
paths are relative to `scripts/`. If a citation and the code disagree, the code wins:
run `deploy-camunda --help` on a freshly built binary.

## 1. Build the binary

```bash
make install.dx-tooling        # or: cd scripts/deploy-camunda && go build -o deploy-camunda .
```

## 2. Choose the inputs

| Input | Flag | Source |
|---|---|---|
| Chart directory | `--chart-path charts/camunda-platform-<v>` (do not combine with `--version`) | `deploy-camunda/cmd/root.go:88-90` |
| Scenario | `--scenario <registry name>`, the file name without `.yaml` under `charts/<v>/test/ci/registry/scenarios/` | `deploy-camunda/cmd/root.go:94`, `deploy-camunda/README.md` "Scenario catalog" |
| Namespace / release | `--namespace` / `--release` | `deploy-camunda/cmd/root.go:91,93` |
| Kube context | `--kube-context "$CTX"`. Always pass it; do not rely on the current context. | `deploy-camunda/cmd/root.go:139` |
| Platform | `--platform gke` (default `gke`; also `eks`, `rosa`) | `deploy-camunda/cmd/root.go:97` |
| Helm timeout | `--timeout <minutes>` (default 5) | `deploy-camunda/cmd/root.go:129` |
| Non-interactive | `--interactive=false`. Agents must set it, because the default `true` prompts for missing variables. | `deploy-camunda/cmd/root.go:113` |

Simplest scenario to start with: `elasticsearch-basic` (basic auth, no Keycloak, one
Elasticsearch companion chart), as defined in
`charts/camunda-platform-8.10/test/ci/registry/scenarios/elasticsearch-basic.yaml`. A matching
registry scenario supplies identity, persistence and features (`deploy-camunda/README.md`, the
paragraph about registry declarations).

## 3. Ingress host and the domain allow-list

`base.yaml` substitutes `${CAMUNDA_HOSTNAME}`, so you need one of these:

- `--ingress-hostname <full host>`. It is not checked against any list, and it cannot be
  combined with the two flags below (`deploy-camunda/config/merge.go:489-491`).
- `--ingress-base-domain <zone>` plus an optional `--ingress-subdomain`. If you omit the
  subdomain, the namespace is used as the subdomain (`deploy-camunda/cmd/root.go:126-127`). The zone **must** be one of
  the hard-coded allow-list `config.ValidIngressBaseDomains` (`deploy-camunda/config/config.go:30-34`):
  - `ci.distro.ultrawombat.com` (GKE `distro-ci`)
  - `distribution.aws.camunda.cloud` (EKS)

  Any other value fails with `--ingress-base-domain must be one of: ...`
  (`deploy-camunda/config/merge.go:495-498`). On `main` you cannot extend this list without
  changing the code. Draft PR #7962 wraps the list in a `config.DomainPolicy` but keeps the
  same defaults.

## 4. Registry credentials (prerequisite for Harbor images)

The scenario values reference the pull secrets `registry-camunda-cloud` (Harbor) and
`index-docker-io` (Docker Hub), for example in
`charts/camunda-platform-8.10/test/integration/scenarios/chart-full-setup/values/base.yaml`.
`deploy-camunda` creates these secrets only when you ask it to:

- `--ensure-docker-registry` creates `registry-camunda-cloud` for host `registry.camunda.cloud`
  (`camunda-core/pkg/kube/kube.go:342`, `deploy-camunda/credentials/store.go:28`).
- `--ensure-docker-hub` creates the Docker Hub secret.
  The flags are defined in `deploy-camunda/cmd/flag_groups.go:80-90`.

When a flag is set, the credentials resolve in this order. The first source that is set wins
(`deploy-camunda/cmd/credentials.go:118-171`):

1. Flags: `--docker-username/--docker-password` (Harbor), `--dockerhub-username/--dockerhub-password`.
2. Environment variable pairs, in order. For Harbor: `HARBOR_USERNAME/HARBOR_PASSWORD`, then
   `TEST_DOCKER_USERNAME_CAMUNDA_CLOUD/..._PASSWORD`, then `NEXUS_USERNAME/NEXUS_PASSWORD`. For Docker Hub:
   `DOCKERHUB_USERNAME/DOCKERHUB_PASSWORD`, then `TEST_DOCKER_USERNAME/TEST_DOCKER_PASSWORD`. If a pair
   is only half set, the command fails.
3. The OS keyring, which `deploy-camunda credentials configure --registry harbor|dockerhub` fills
   (`deploy-camunda/cmd/credentials.go:40-105`). Check it with `deploy-camunda credentials status`.

Without either `--ensure-docker-*` flag, no credentials are read and no pull secret is created.
Public `docker.io/camunda/*` images still pull anonymously, but they are rate-limited.
Harbor-only images (for example SNAPSHOT tags) then land in `ImagePullBackOff`.
The preflight and `doctor` only *fail* on missing registry credentials when the matching
`--ensure-docker-*` flag is set. Otherwise the check is only a warning (`deploy-camunda/deploy/preflight.go:283-319`). Never try to
extract credentials yourself. Ask the user.

## 5. External Secrets Operator (ESO): present, absent, or not usable

`--external-secrets` defaults to **true** on the root command (`deploy-camunda/cmd/root.go:100`).
Before Helm runs, the deployer calls `kube.ApplyExternalSecretsAndCerts`
(`deploy-camunda/pkg/deployer/deployer.go:117-121`). The outcome depends on the cluster:

| Cluster state | What happens | Code |
|---|---|---|
| The ESO CRD `externalsecrets.external-secrets.io` is **not installed** | The deployer logs a warning, skips the step and continues. TLS and `integration-test-credentials` are then missing. | `camunda-core/pkg/kube/kube.go:838-845` |
| The CRD is installed, but the store is wrong or unreachable (for example `vault-backend` on `distro-ci`, whose store is `distribution-team`) | The ExternalSecrets are applied, then each one is polled every 5 s for up to **600 s** before the deploy fails with `wait for ExternalSecrets ready`. Helm never runs. | `camunda-core/pkg/kube/platforms.go:107-110,201-204,209`, `kube.go:1082-1112` |
| The namespace may not reference the ClusterSecretStore (restricted sandboxes) | Applying the ExternalSecret is refused and the deploy stops. | — |

When ESO is absent or unusable, deploy without it:

- Pass `--external-secrets=false` to skip the ESO step entirely. This avoids the 600 s hang.
- Pass `--auto-generate-secrets` to generate the test secrets (identity user passwords and the
  Keycloak client secret). The generated secrets are appended to the `--env-file` (default `.env`), and the
  mapped `vault-mapped-secrets` Secret is applied before Helm runs
  (`deploy-camunda/deploy/values.go:761-786`, `deploy-camunda/deploy/secrets.go:100-121`,
  `deploy-camunda/pkg/deployer/deployer.go:129-137`). This **does not** replace the ESO-provided
  TLS certificate or the `integration-test-credentials` Secret. Scenarios that need those (TLS
  ingress, e2e credentials) must be given the secrets by hand.

Do not combine `--use-vault-backed-secrets`/`--external-secrets-store vault-backend` with a cluster
whose store is not `vault-backend`. Check the store with
`kubectl --context "$CTX" get clustersecretstores.external-secrets.io` (this is cluster-scoped, so it may be refused in sandboxes).

## 6. Check, then deploy

Run the read-only checklist first (`deploy-camunda/cmd/doctor.go:37-145`):

```bash
deploy-camunda doctor \
  --chart-path charts/camunda-platform-8.10 \
  --scenario elasticsearch-basic \
  --kube-context "$CTX"
```

Optional: render only, without touching the cluster. This resolves the same values but skips hooks and
companion charts (`deploy-camunda/cmd/root.go:120-121`, `deploy-camunda/README.md`):

```bash
deploy-camunda --chart-path charts/camunda-platform-8.10 --scenario elasticsearch-basic \
  --namespace "$NS" --release integration --ingress-hostname "$NS.ci.distro.ultrawombat.com" \
  --interactive=false --render-templates --render-output-dir ./rendered
```

Deploy to GKE `distro-ci` with ESO available and Harbor credentials in the environment:

```bash
CTX=gke_camunda-distribution_europe-west1-b_distro-ci
NS=<your-namespace>
deploy-camunda \
  --chart-path charts/camunda-platform-8.10 \
  --scenario elasticsearch-basic \
  --platform gke \
  --namespace "$NS" --release integration \
  --kube-context "$CTX" \
  --ingress-base-domain ci.distro.ultrawombat.com \
  --ensure-docker-registry \
  --interactive=false \
  --timeout 15
```

If ESO is absent or unusable (section 5), add `--external-secrets=false --auto-generate-secrets`.
If you have no Harbor credentials, drop `--ensure-docker-registry` and expect only public images to pull.

Other flags you may need: `--ttl 8h` for the namespace janitor (`root.go:111`), `--delete-namespace` to
start clean (`root.go:118`), `--skip-preflight` to bypass the fail-fast preflight (`root.go:114`),
`--extra-values f.yaml` (applied last, `root.go:122`), and `--image-override component=ref` (`root.go:123`).
Watch the deploy from a second terminal with `deploy-camunda watch` (see `SKILL.md`).

## 7. Not yet available. Do not use.

These commands are **not on `main`**. The binary rejects them with `unknown command` or `unknown flag`.

| Capability | Status | PR |
|---|---|---|
| `deploy-camunda scenario list` | Not built. No open PR adds the command. The nearest is draft **#7985**, which adds an internal `Recipe` catalogue built from the registry, with no CLI wiring. Use `deploy-camunda matrix list --repo-root . --versions <v>` or `ls charts/<v>/test/ci/registry/scenarios/` instead. | none yet (#7985 is groundwork) |
| `deploy-camunda scenario deploy` | Not built. No open PR adds the command. Use the root command (section 6) or `matrix run`. | none yet |
| `deploy-camunda doctor --format json` | Not on `main`. Text output only. | draft **#7983** |
| The target-design config file (versioned `deploy-camunda/v1`) | Not built. No open PR adds it. Today's `.deploy-camunda.yaml` / `~/.config/camunda/deploy.yaml` profile file **does** work (`--config`/`-F`, `root.go:84`; `deploy-camunda config ...`). | none yet |
