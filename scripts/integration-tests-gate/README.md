# Integration Tests Gate

Required status check that wraps the `Test - Chart Version` matrix
workflow with a one-shot retry of failed jobs.

The merge queue gates on this workflow, not on the underlying matrix.
A transient failure in a single matrix cell does not evict the PR:
the gate retries failed cells once and only reports its own final
conclusion.

## Behavior

- One retry **per gate invocation**. Re-running the gate workflow
  yields one additional retry; useful for recovering from a
  double-transient without pushing a new commit.
- `cancelled` / `timed_out` / `action_required` are **not** retried.
  Only `conclusion == failure` triggers `gh run rerun --failed`.
- The gate's required check is `Integration Tests Gate / gate`.
  Branch protection / merge-queue config must require this check and
  not the raw matrix check.

## Fork PRs

The gate is skipped on PRs from fork repositories. `GITHUB_TOKEN`
on fork PRs has no `actions: write` scope, so `gh run rerun --failed`
would 403. For fork PRs, the matrix workflow's own status is the
required signal.

## Manual debugging

Use `workflow_dispatch` to run the gate against a specific SHA:

```bash
gh workflow run integration-tests-gate.yaml \
  -f sha=<commit-sha> \
  -f event=pull_request
```

## Development

```bash
cd scripts/integration-tests-gate
go test ./...
go vet ./...
go build .
```

The gate logic lives in `gate.go` behind a `ghClient` interface;
`gh.go` is the production implementation that shells out to the
`gh` CLI. `gate_test.go` uses a fake client to exercise the state
machine without touching the GitHub API.
