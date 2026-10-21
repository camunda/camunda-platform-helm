---
name: chart-minor-cut
description: Cut a new Camunda Helm chart minor, for example charts/camunda-platform-8.12 at 17.0.0-alpha0. Covers hardcoded-version discovery, the git ls-files copy, image tags, the copy audit, the wiring, regeneration, checks, the bootstrap tag and the lifecycle of older minors. Use when asked to create the next chart minor or to cut a new alpha chart line.
---

# Cut a New Chart Minor

Use these names in every step:

| Name | Meaning | Example |
|---|---|---|
| `N` | The new Camunda minor | `8.12` |
| `P` | The previous minor. The new chart is a copy of `P`. | `8.11` |
| `M` | The new chart major | `17` |
| `NEW` | `charts/camunda-platform-N` | `charts/camunda-platform-8.12` |
| `OLD` | `charts/camunda-platform-P` | `charts/camunda-platform-8.11` |

Do the steps in sequence. Each step has a check. Do not go to the next step when a check fails.

The human checklist for the same steps is in `docs/reference/release-process.md`, section "New alpha chart". Copy it into the tracking issue of the cut.

## 1. Read the state of P

1. Read `charts/chart-versions.yaml`.
2. Find the `P` entry in `camundaSupportLifecycle`.
3. If the entry has `released`, `P` is GA. Record this value. Step 7 uses it.

Check: this command shows the `P` entry. A `released:` line means that `P` is GA.

```bash
sed -n '/^  "P":/,/^  "/p' charts/chart-versions.yaml
```

## 2. Make tooling version-agnostic first

Hardcoded lists of `P` break the new chart line. Find them before you copy the chart.

```bash
git grep -nE "[\"']P[\"']|camunda-platform-P([^0-9]|$)|SM-P([^0-9]|$)" -- \
  ':!charts/camunda-platform-*' ':!version-matrix' ':!**/CHANGELOG.md'
```

Replace `P` with the real value, with escaped dots (for example `8\.11`). Examine each hit:

- A numeric comparison (for example `compareVersions(v, "8.10") < 0`) is correct. Keep it.
- An example in a document is correct. Keep it.
- A substring check or a hardcoded list is a defect. Change it to a numeric comparison. If GitHub Actions expressions make that impossible, add `N` to the list.

Put these changes in a separate PR that merges first. In the 8.11 cut, the changes were:

- `scripts/prepare-helm-values/pkg/values/processor.go`: license key shape (`usesLicenseSecret`).
- `.github/workflows/test-local-template.yaml`: ECK install and appProtocol conditions.
- `test/e2e/playwright.base.config.ts`: `ShadowVersion`.
- `helm-values-mcp/test/helm-values.test.ts`: the latest-version assertions.
- `scripts/validate-values-schema/upgrade_test.go`: the `expectations` map has one key for each chart that has `test/unit/deprecation/allowlist.yaml`. Add a `camunda-platform-N` key with the same keys as `P`. The 8.11 cut missed this, and `Go test + vet` failed on every PR until #7578.

Check: run the grep again after the changes. It shows no hardcoded list that excludes `N`.

## 3. Copy the chart

Copy the tracked files only. Do not use `cp -r`. It copies ignored files (`Chart.lock`, `charts/`, `node_modules/`), and a local exclude file can drop tracked files.

```bash
git ls-files -z OLD | while IFS= read -r -d '' f; do
  d="NEW/${f#OLD/}"; mkdir -p "$(dirname "$d")"; cp -p "$f" "$d"
done
```

Check: the two commands below show the same number, and `NEW/test/e2e/.env.template` exists.

```bash
git ls-files OLD | wc -l
find NEW -type f | wc -l
```

## 4. Edit the chart metadata

1. In `NEW/Chart.yaml`, set `version: M.0.0-alpha0` and `appVersion: N.x`.
2. Add `artifacthub.io/prerelease: "true"` as the first annotation. The GA chores of `P` removed it.
3. Set `artifacthub.io/changes` to one entry: `kind: added`, `description: "Initial N alpha chart"`.
4. Replace the content of `NEW/CHANGELOG.md` with `# Changelog`.
5. Replace the content of `NEW/RELEASE-NOTES.md` with this line: `The changelog is automatically generated and it follows [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) format.`

Check: this command shows `M.0.0-alpha0`, `N.x` and `"true"`, and no `P` version.

```bash
grep -nE '^(version|appVersion):|prerelease' NEW/Chart.yaml
```

## 5. Replace the image tags

The copy has the GA tags of `P`. Replace each Camunda image tag in `NEW/values.yaml`, `NEW/values-latest.yaml` and `NEW/values-digest.yaml`.

1. For each image, query Docker Hub for a versioned snapshot tag:

   ```bash
   curl -s -o /dev/null -w '%{http_code}\n' \
     "https://hub.docker.com/v2/repositories/camunda/<image>/tags/N-SNAPSHOT"
   ```

2. If the result is `200`, use `N-SNAPSHOT`. If the result is `404`, use `SNAPSHOT`. In the 8.11 cut, `camunda/identity`, `camunda/hub` and `camunda/hub-websockets` had no versioned snapshot tag.
3. Fetch the digest from the selected tag's URL, replacing `<selected-tag>` with `N-SNAPSHOT` or `SNAPSHOT`. Set that image's `digest` in `NEW/values-digest.yaml` to the output. Stop if the command fails.

   ```bash
   set -o pipefail
   curl -fsS "https://hub.docker.com/v2/repositories/camunda/<image>/tags/<selected-tag>" |
     jq -er '.digest | strings | select(length > 0)'
   ```

4. In `NEW/values-latest.yaml`, set line 2 to `# Camunda - Alpha`.

Check: each pinned digest matches its selected-tag lookup result, and this grep shows no hits.

```bash
grep -nE 'tag: "?P\.[0-9]' NEW/values.yaml NEW/values-latest.yaml NEW/values-digest.yaml
```

## 6. Audit the copy

Find text that the copy brought from older cycles:

```bash
grep -rnE 'P|P-1|camunda-platform-8\.[0-9]+' NEW \
  --exclude-dir=testdata --exclude-dir=golden \
  --exclude=values.schema.json --exclude=values.unknown-keys.schema.json \
  --exclude=registry-snapshot.yaml
```

Use the real values of `P` and `P-1`, with escaped dots. The command skips the generated files. Step 8 regenerates them. Hits in the parameter table of `NEW/README.md` are also generated. Ignore them in this step.

Sort each hit into one of these groups:

| Group | Action |
|---|---|
| A history statement, for example "removed in P" or "deprecated since P" | Keep it. |
| A docs link to `docs.camunda.io/docs/P/` | Change `P` to `next`. Check that the `next` page exists with `curl -L`. |
| `camundaPlatform.ReleaseHighlights` in `NEW/templates/common/_helpers.tpl` | Change the text to "from P to N". Find the upgrade guide in `camunda/camunda-docs` under `docs/self-managed/upgrade/helm/`. If no `P`-to-`N` page exists, use `https://docs.camunda.io/docs/next/self-managed/upgrade/helm/`. Do not guess a URL. |
| A chart directory name in the hand-written part of `NEW/README.md` | Change it to `camunda-platform-N`. |
| `NEW/test/e2e/playwright.config.ts` | Change `SM-P` to `SM-N`. Keep the empty-directory fallback until QA publishes the `SM-N` suite. Remove every `throw` that a `REQUIRE_*_TEST_SUITE` variable triggers, and add a skip spec for each project entry point. In the 8.11 cut, the physical-tenants throw failed the `ptnt` merge-queue scenario. |
| The first-line header of a test values file | Change the version to `N`. |
| A Hub cluster `version:` for an Orchestration Cluster that this chart installs | Change it to `N-SNAPSHOT`. Keep versions that describe older clusters on purpose. |
| A path under `OLD` in `NEW/test/ci/registry/` | Change it to `NEW`. |
| A hook or scenario for the `P-1` to `P` upgrade, for example a Bitnami migration | Remove the hook files, the scripts and the `post-infra:` lines. A `P` installation does not have the old state. |

Check: run the audit command again. Each remaining hit is in the "Keep it" group.

## 7. Wire the new minor

Make these changes outside the chart directory:

| File | Change |
|---|---|
| `charts/chart-versions.yaml` | Add `"N"` at the top of `chartAutomation.routineVersions`. Add `"N": {}` at the top of `camundaSupportLifecycle`. Do not change other minors. |
| `.github/config/release-please/release-please-config.json` | Add a package for `NEW`. Copy the `P` package. Set `component: camunda-platform-N`, the title suffix `Camunda N`, and `extra-label` that starts with `version/alpha,version/N`. |
| `.github/config/release-please/.release-please-manifest.json` | Add `"NEW": "M.0.0-alpha0"` and `"NEW+FILLER": "0.0.0"`. |
| `.github/renovate.json5` | Add `NEW/Chart.yaml` to the alpha `github-tags` rule. Add `NEW/Chart.yaml`, `NEW/values.yaml` and `NEW/values-latest.yaml` to the alpha images group and to the alpha semver rule. Add `NEW/values-digest.yaml` to both digest rules. Do not add a group for one version only. |
| `.github/config/pr-labeler.yaml` | Add `version/N` for `NEW/**`. |
| `.github/config/permitted-flows.yaml` | Add `==N` with `deny: [upgrade-patch]`. If `P` is not GA, also deny `upgrade-minor`, and remove the `upgrade-minor` flow from the scenarios in `NEW/test/ci/registry/`. The registry validation fails on a scenario flow that this file denies. When `P` becomes GA, remove the `upgrade-minor` deny for `N` and add the `upgrade-minor` flow back to those scenarios. Without this follow-up, `N` has no minor-upgrade coverage. |
| `.github/workflows/test-chart-version.yaml` | Add `"N"` to the version choices. |
| `.github/workflows/test-release-tooling.yaml` | Add `N` to the chart loop. |
| `scripts/release-tools/inject_values.go` | Add `N` to `validVersions` and to the error text. Check the component set in the `mergeImageTags` switch. |
| `scripts/render-e2e-env.sh` | Make a bare `SNAPSHOT` version resolve to `SM-N`. |
| `docs/reference/release-process.md` | Add `M.x` = Camunda `N` to the version map. |

Create the label:

```bash
gh label create "version/N" --repo camunda/camunda-platform-helm \
  --color fc5d0d --description "Camunda applications/cycle version"
```

Check: these commands exit with code 0, and the `git grep` shows `N` in each file of the table.

```bash
python3 -m json.tool .github/config/release-please/release-please-config.json > /dev/null
python3 -m json.tool .github/config/release-please/.release-please-manifest.json > /dev/null
git grep -c 'N' -- charts/chart-versions.yaml .github/config .github/renovate.json5 \
  .github/workflows/test-chart-version.yaml .github/workflows/test-release-tooling.yaml \
  scripts/release-tools/inject_values.go scripts/render-e2e-env.sh docs/reference/release-process.md
```

## 8. Regenerate the artifacts

```bash
make helm.dependency-update chartPath=NEW
make helm.readme-update chartPath=NEW
make helm.schema-update chartPath=NEW
make go.update-golden-only chartPath=NEW
```

`go.update-golden-only` also updates `NEW/test/ci/registry-snapshot.yaml`.

`helm.schema-update` needs a working `jq`. If `jq --version` prints nothing, the asdf install can have the wrong CPU architecture. Install the release binary for your platform at the version in `.tool-versions`. Check the release checksum before you install it.

Check: this diff shows only the tags and links that you changed. Then run the step 6 audit again without the `--exclude` options for the generated files.

```bash
diff <(python3 -m json.tool OLD/values.schema.json) <(python3 -m json.tool NEW/values.schema.json)
```

## 9. Run the checks

Run each command. Each command must pass.

```bash
make helm.lint chartPath=NEW
make go.test chartPath=NEW
for m in $(find scripts -name go.mod -exec dirname {} \;); do
  (cd "$m" && go test ./...)
done
make mcp.test
```

If a unit test in `NEW` fails on a value that the copy hardcoded to `P`, change the expectation to `N`. In the 8.11 cut, these tests needed changes:

- `test/unit/connectors/deployment_test.go`: the `app.kubernetes.io/version` label.
- `test/unit/web-modeler/configmap_restapi_test.go`: the `N-SNAPSHOT` management-cluster fallback, and a pinned SemVer identity tag.

Check the first dev version:

```bash
(cd scripts/release-tools && go build -o /tmp/release-tools .)
GITHUB_ENV=/dev/stdout /tmp/release-tools release-version \
  --current-version M.0.0-alpha0 --chart-version N \
  --chart-versions-file charts/chart-versions.yaml --chart-dir NEW --short-sha abc1234
```

The output must contain `RELEASE_VERSION=M.0.0-alpha1`.

Check the rendered images:

```bash
grep -rhoE --include='*.golden.yaml' 'image: "?[^" ]*camunda/[a-z-]+[:@][^" ]+' NEW/test/unit | sort | uniq -c
```

The command reads the rendered goldens only. Each image must use the tags from step 5.

Check that `OLD` did not change: `git status --short OLD` shows nothing.

Run the tier-1 scenarios for `N` with the `rfr-validation` skill.

## 10. Open the PR

1. Use the title `feat: create the Camunda N Helm chart line as M.0.0-alpha0`.
2. Fill in `.github/pull_request_template.md`.
3. If the tooling PR from step 2 is not merged, stack this PR on it with the `gh-stack` skill. GitHub does not link closing keywords on a PR whose base is not `main`. Link the issue with the `addCloseIssueReferences` GraphQL mutation.

Check: the first command shows the issue number. If the PR is stacked, the second command does not show `null`.

```bash
gh pr view <pr> --repo camunda/camunda-platform-helm --json closingIssuesReferences \
  --jq '[.closingIssuesReferences[].number]'
gh api repos/camunda/camunda-platform-helm/pulls/<pr> --jq '.stack'
```

## 11. After the merge

1. Create the bootstrap tag on the merge commit at once. The release notes and the PR version labels read it. Without this tag, every dev build on `main` fails at "Generate RELEASE-NOTES.md" (`git show-ref --hash camunda-platform-N-M.0.0-alpha0: exit status 1`).

   ```bash
   git tag -a camunda-platform-N-M.0.0-alpha0 <merge-sha> -m "Create the N chart line"
   git push origin camunda-platform-N-M.0.0-alpha0
   ```

2. Check that the first dev build of `NEW` makes `M.0.0-alpha1`.
3. Forward-port the changes to `OLD` that merged after the copy. List them with `git log <copy-sha>..origin/main -- OLD`, and open one PR for each that applies the same diff to `NEW`. For open PRs that change `OLD`, add the `forward-port-to-N` label. A bot author cannot act on a review, so do the change in the bot PR yourself.
4. Tell QA that `NEW` exists. QA publishes the `SM-N` e2e suite. Then remove the fallback in `NEW/test/e2e/playwright.config.ts`.
5. Tell the monorepo team that the `N` preview-environment smoke test can run.

Check: the first command shows one tag line. Use the second command to locate the run that first published `NEW`.

```bash
git ls-remote --tags origin camunda-platform-N-M.0.0-alpha0
gh run list --repo camunda/camunda-platform-helm --workflow chart-build-dev.yaml --branch main --limit 5
```

For that run, the `Build N` job must have `conclusion: success`. Use its `databaseId` as `<job-id>`. The final command must show a published dev tag beginning with `M.0.0-alpha1-dev-`.

```bash
set -o pipefail
gh run view <run-id> --repo camunda/camunda-platform-helm --json jobs \
  --jq '.jobs[] | select(.name == "Build N") | {databaseId, conclusion}'
gh run view <run-id> --repo camunda/camunda-platform-helm --job <job-id> --log |
  grep -F '##[notice]Pushed to Harbor with tags: '
```

## 12. Plan the lifecycle of older minors

Do not change other minors in the cut PR. Open separate PRs.

1. Find the end of standard maintenance of each minor in the docs release announcements.
2. Find each ESUP case in the handbook ESUP Case Registry on Lamppost. Do not put customer names in public issues or PRs.
3. For a minor past its end of standard maintenance that still has `stdSupportUntil`, remove `stdSupportUntil`. Keep it in `routineVersions` only if an ESUP case includes Helm charts.
4. For a minor past its end of standard maintenance with no ESUP case, open one EOL PR. In that PR:
   - Remove the chart directory.
   - Set `eolSince` and `latestChart`.
   - Remove the minor from release-please, `pr-labeler.yaml`, `renovate.json5`, the `Makefile` and `.github/AGENTS.md`.
   - Regenerate `version-matrix/camunda-<minor>/README.md` and `version-matrix/README.md` with `release-tools version-matrix`.
   - Remove `version-matrix/camunda-<minor>/version-matrix.json`.

Check: in the cut PR, this diff adds lines for `N` only. Other minors do not change.

```bash
git diff origin/main -- charts/chart-versions.yaml
```

## Known failures from earlier cuts

| Do not | Incident |
|---|---|
| Copy with `cp -r` or with a local exclude file | #4469 and #5543: files were missing from the new chart |
| Keep the image tags of `P` | #4598 and #5889: the new chart ran the `P` images |
| Keep `component` from the copied release-please package | bd9634738f: release-please used the wrong component |
| Forget the manifest filler entry | #5334 |
| Add a Renovate group for one version | #4543: about 11 Renovate PRs in one week |
| Change the lifecycle of other minors in the cut PR | #5526 and #5754: 8.6 left routine CI too early |
| Merge the cut with failing checks | #4463: CI stayed red until #4503 |
| Forget the bootstrap tag | The first 8.9 alpha release failed 29 days after the cut. In 8.11, every dev build on `main` failed until the tag was pushed |
| Skip the scripts modules that do not run on the cut PR | #7578: `upgrade_test.go` had no 8.11 entry |
| Keep a `REQUIRE_*_TEST_SUITE` throw while `SM-N` is unpublished | The 8.11 `ptnt` scenario failed in the merge queue |
| Requeue a stacked cut PR with `gh pr merge` | It fails with "Auto-merge is not supported for stacked pull requests". Use `gh stack merge <pr> --yes` after the required checks report |
