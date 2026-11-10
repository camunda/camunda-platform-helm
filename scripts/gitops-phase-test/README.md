# GitOps phase test

This harness checks the Camunda Hub `normal -> quiesce -> migrate -> normal`
lifecycle through Helm, Argo CD, and Flux. It uses lightweight test containers,
not a Hub database migration.

Argo CD registers the repository supplied by `--repo` with `depth: "1"` before
creating the Application. The repo-server fetches only the tested revision,
without downloading all branches, tags, and history. The Application remains
pinned to `--revision`, and the five-minute phase timeout is unchanged.

## CI

Pull requests run Helm and Flux. Merge-queue and manual runs also run Argo CD.
Each controller gets a fresh KIND cluster, which is deleted after the test.
The Argo CD install verifies that Redis uses the authenticated Docker Hub image.

Flux uses `refs/heads/<branch>` for same-repository pull requests and merge-queue
runs. Fork pull requests retain `refs/pull/<number>/head` in the base repository.
The harness rejects a Flux artifact whose revision differs from `--revision`.

To run all three controllers against a branch:

```bash
gh workflow run test-gitops-phase.yaml \
  --repo camunda/camunda-platform-helm --ref <branch>
```

Manual runs print Argo CD diagnostics on success and failure. The repo-server
log shows the tested commit in `git fetch origin <commit> --depth 1 --force --prune`.
Run the workflow again at the same commit to check another cold fetch.

Unit tests run in `make go.test-scripts` through the Test - Release Tooling
workflow.
