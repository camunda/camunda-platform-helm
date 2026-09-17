# Autofix Publisher Tests

Run `go test -race -count=1 ./...` in this directory. The suite loads the actual
publisher workflow scripts and executes them against disposable local Git
repositories. It uses a dummy token and never contacts GitHub or Vault.

Chart chores and Renovate post-upgrade generate commits on read-only runners.
Only a Git bundle crosses into the shared publisher. Its fresh runner imports
objects into a bare repository, without checking out candidate files or restoring
caches. Before retrieving App credentials from Vault it checks:

- The bundle is at most 50 MiB and contains a nonempty commit with exactly one
  parent, the immutable source event SHA.
- Every changed path is an allowed generated output: chart README, schema, lock,
  golden YAML, or registry snapshot. Renovate additionally permits module files
  under `charts/` and `scripts/`.
- New and modified files are regular, nonexecutable files; deletions are allowed.
- The target branch still points to the source SHA.

The publisher creates a repository-scoped, contents-write App token only after
validation. A per-command credential helper supplies it to Git, with hooks
disabled. An explicit expected-SHA lease closes the race after the branch check.
Because the candidate is a direct child of that exact SHA, an accepted update is
a fast-forward; concurrent commits are never overwritten. The bundle preserves
the generated commit, including binary content and deletions.

If the branch moved, rerun generation against its new head. Rerun the whole
workflow, not just the publisher: artifact names include the run attempt. An
unchanged tree creates no artifact and skips the publisher. Manual dispatch is
available for same-repository maintenance branches; Renovate dispatch still
requires a `renovate/` branch. Publishing to `main`, `stable/`, and release-please
branches is refused.

## Trust Boundary

The split isolates repository/dependency code executed during generation from
the credential-bearing runner. Artifact contents remain untrusted; the path and
history checks constrain what this automation can publish, not whether generated
content is semantically correct. Normal PR review and checks still apply.

The workflow definitions, pinned actions, runner tools, and existing Vault
authorization remain trusted. This change does not protect against an actor who
can rewrite a secret-enabled workflow to request credentials directly. Restricting
Vault authentication to a protected publisher definition is a separate policy
change. Neither producer executes on `pull_request_target`, and fork PRs do not
enter the autofix/publish path.