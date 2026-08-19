# Integration Tests Gate

Required status check that wraps the `Test - Chart Version` matrix
workflow with a one-shot retry.

The merge queue gates on this workflow, not on the underlying matrix.
A transient failure in a single matrix cell does not evict the PR:
the gate retries the run once and only reports its own final
conclusion.

The retry re-runs the **whole** run, not just the failed jobs. Cells
that already passed short-circuit through the scenario result cache
(`Cached pass`), so a full re-run costs little more than a partial one,
and jobs that depend on cluster state created by an earlier job in the
same run — `upgrade`, `playwright-e2e-*`, `shadow-e2e` — get that state
recreated. Re-running those jobs alone cannot work: the `Cleanup` job
of the previous attempt annotates the test namespace `cleaner/ttl=1s`,
so the retried job would fail with `namespace ... not found` and mask
the original error.

## Behavior

- One retry **per gate invocation**. Re-running the gate workflow
  yields one additional retry; useful for recovering from a
  double-transient without pushing a new commit.
- `cancelled` / `timed_out` / `action_required` are **not** retried.
  Only `conclusion == failure` triggers `gh run rerun`.
- The gate's required check is `Integration Tests Gate / gate`.
  Branch protection / merge-queue config must require this check and
  not the raw matrix check.

## continue-on-error jobs

The gate looks at the run-level `conclusion`, never per-job. GitHub
treats `continue-on-error: true` jobs as soft failures: their internal
steps can fail but the job's `conclusion` stays `success` and they do
not contribute to the run-level conclusion. Practical consequences:

- A run with only soft (continue-on-error) failures is `success` at the
  run level. The gate exits 0 without retrying.
- A run with a mix of hard and soft failures is `failure` at the run
  level. The gate retries the whole run, so soft-failing jobs re-run
  too, but their result still cannot make the gate fail.

If you want a flaky job to gate the merge queue at all, do NOT mark it
`continue-on-error: true` — a run whose only failures are soft is
`success` at the run level and is never retried.

## Fork PRs

The gate is skipped on PRs from fork repositories. `GITHUB_TOKEN`
on fork PRs has no `actions: write` scope, so `gh run rerun`
would 403. For fork PRs, the matrix workflow's own status is the
required signal.

## Manual debugging

Use `workflow_dispatch` to run the gate against a specific SHA:

```bash
gh workflow run integration-tests-gate.yaml \
  -f sha=<commit-sha> \
  -f event=pull_request
```

GitHub Actions retains run history for 90 days; dispatching against a
SHA older than that will fail discovery because the matrix run has
been pruned.

## Development

```bash
cd scripts/integration-tests-gate
go test ./...
go vet ./...
go build .
go run .   # how the workflow invokes the gate
```

The gate logic lives in `gate.go` behind a `ghClient` interface;
`gh.go` is the production implementation that shells out to the
`gh` CLI. `gate_test.go` uses a fake client to exercise the state
machine without touching the GitHub API.
