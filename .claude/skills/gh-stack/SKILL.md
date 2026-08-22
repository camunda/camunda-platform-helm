---
name: gh-stack
description: Split a large change into GitHub native stacked pull requests and manage them with the gh stack extension — create or adopt a stack, cascade-rebase it, keep it in sync, and land it bottom-up with an atomic stack merge. Use when a change is too large for one PR, when asked to stack or restack PRs, when a PR's base is another feature branch, or when maintaining a chain of dependent PRs in this repo.
---

# GitHub Native Stacked Pull Requests

A **stack** is a chain of two or more PRs in one repository: the bottom targets `main`, and each
one above targets the branch of the PR below it. GitHub tracks the stack as a first-class object
and renders a stack map on every member, so reviewers can see the layering without reading each
body.

Use a stack when a change is large enough that one PR would be hard to review but the parts are
genuinely dependent. For one or two independent PRs, don't — a plain branch off `main` is simpler.

## Prerequisites

```bash
gh extension install github/gh-stack
gh stack --version
```

Docs: <https://docs.github.com/en/pull-requests/how-tos/stacked-pull-requests>

Constraints worth knowing before you start:

- **Same repository only.** Cross-fork stacks are not supported, so this works for maintainers
  pushing branches to `camunda/camunda-platform-helm`, not for fork-based contributions.
- **Merge bottom-up.** A PR cannot land before the ones below it.
- GitHub Desktop does not support stacks.

## Setting a base branch does NOT create a stack

This is the trap. Creating a PR whose `--base` is the branch below it produces a correct base
chain and *no stack*: the PR reports `.stack == null` and gets no stack map. The stack is an
explicit object you create or join.

```bash
# Chained bases only — NOT yet a stack:
gh pr create --draft --base <branch-below> --head <my-branch> --title "..." --body-file body.md

# Then actually put it in the stack:
gh stack link <stack-number> <pr-number>
```

Verify, don't assume:

```bash
gh api /repos/camunda/camunda-platform-helm/pulls/<n> --jq '.stack'
# -> {"number":<stack>,"position":6,"size":6,...}   or  null
```

## Starting a stack

Two entry points, depending on whether the branches already exist.

**From scratch, building up as you go:**

```bash
gh stack init                  # start a stack on the current branch, targeting the default branch
# ... commit work ...
gh stack add <next-branch>     # add a branch on top of the current stack
gh stack submit                # push all branches, create/update the PRs, and create the stack
```

`gh stack submit` opens an editor to set each PR's title, body, and draft state. Pass `--auto`
(or run non-interactively) to accept auto-generated titles — avoid that here, because this repo
enforces Conventional Commit PR titles (see below).

**From branches that already exist:**

```bash
gh stack init branch1 branch2 branch3   # adopt existing branches, bottom to top
```

## Adopting PRs that already exist

`gh stack link` works without local tracking state, which makes it the right tool for a stack
that was built by hand with chained bases.

```bash
# Create a stack from existing PRs, listed bottom to top:
gh stack link 6884 6889 6891 6893 6902

# Or append to an existing stack — pass the stack number first, then only what's new:
gh stack link 6894 6904
```

Arguments may be branch names, PR numbers, or PR URLs. Arguments already in the stack are
skipped and **existing PRs are never removed**, so re-running is safe. A numeric first argument
is treated as a stack number only when a stack with that number exists; stack and PR numbers
never overlap.

## Daily loop

```bash
gh stack view                  # branches + PR status; --short for one line each, --json to script it
gh stack sync                  # the one command to reach for after main moves
gh stack rebase                # cascading rebase only, no PR-state sync; --no-trunk to skip trunk
gh stack checkout <stack|pr|url|branch>
gh stack up / down / top / bottom / switch / trunk
```

`gh stack sync` fetches, reconciles the local stack against GitHub, fast-forwards trunk,
cascade-rebases each branch onto its updated parent, force-pushes atomically with
`--force-with-lease --atomic`, syncs PR state, and links the open PRs into a stack when two or
more exist.

**Use `gh stack sync` or `gh stack rebase` instead of rebasing each branch by hand.** Rebasing a
stack manually means picking the right old base for every branch, and getting it wrong silently
drops commits — in this repo that has already cost a lost feature commit that had to be recovered
by cherry-pick. The cascade does the bookkeeping.

Rebasing rewrites history and force-pushes, which is correct here: this repo forbids merge
commits (see `AGENTS.md`), so `git merge` is never the way to take in upstream changes.

## Restructuring

```bash
gh stack modify      # interactively reorder, insert, or drop layers
gh stack unstack     # remove the stack locally and on GitHub (the PRs stay open)
```

If a layer turns out to be independent, take it out of the stack and retarget it at `main`
rather than leaving reviewers to guess why it is stacked.

## Landing the stack

```bash
gh stack merge                 # current branch's stack
gh stack merge <stack-number>  # a stack you don't have checked out
gh stack merge <pr-number>     # everything up to and including that PR
```

This is GitHub's **atomic** stack merge: every PR up to your chosen one merges in a single
all-or-nothing operation, so if any one cannot merge, none do. Interactively it offers a wizard
to choose how far up to merge and which merge method; `--yes` or a non-interactive terminal
merges the whole selection without prompting.

When a PR merges, GitHub automatically re-targets and rebases the PRs above it, so there is no
need to repoint bases by hand. Merging a mid-stack PR leaves the ones above open, re-targeted at
the stack's base.

**Confirm before relying on it in this repo:** the full ~33-deploy matrix runs in the *merge
queue*, not on the PR (see the `rfr-validation` skill). How an atomic stack merge interacts with
this repo's merge queue and required checks has not been verified here — check with a maintainer
before using `gh stack merge` on a stack that must go through the queue, and fall back to merging
one PR at a time if in doubt.

## Repo conventions that still apply per PR

A stack does not exempt any member from the normal rules in `AGENTS.md` and
`docs/contribution-and-collaboration.md`:

- **Conventional Commit titles, individually.** Each PR is titled for its own diff. `feat:`,
  `fix:`, `refactor:`, `docs:`, and `revert:` are reserved for PRs that change user-facing chart
  files; a PR in the stack that only touches `.github/`, `scripts/`, or tests must use `ci:`,
  `build:`, `chore:`, or `test:` or CI rejects it. It is normal for one stack to mix types.
- **Draft-first, then `crev`.** Open each PR as a draft, run
  `crev https://github.com/camunda/camunda-platform-helm/pull/<n>` against it, address findings,
  then `gh pr ready <n>`. Review the whole stack before landing any of it, because a change
  demanded in a lower PR forces a cascade rebase of everything above.
- **Regenerate artifacts in the PR that changes them**, not in a later layer, or the intermediate
  PR is left with a failing `make go.test`.
- **Do not hand-maintain position markers.** GitHub owns `position` and `size`; a `[TAG N/M]`
  marker in the title or a stack table copied into every body is a second source of truth that
  goes stale the moment the stack grows. Let the stack map speak.

## Inspecting a stack from the API

Useful when scripting or when the UI is not to hand.

```bash
# Every PR carries its stack membership:
gh api /repos/camunda/camunda-platform-helm/pulls/<n> --jq '.stack'

# The stack itself is addressable, with its PRs in order:
gh api /repos/camunda/camunda-platform-helm/stacks/<stack-number> \
  --jq '.pull_requests[] | "#\(.number) \(.head.ref) \(.title)"'

# One-line status for a whole stack:
for n in <prs>; do
  gh api /repos/camunda/camunda-platform-helm/pulls/$n \
    --jq '"\(.stack.position)/\(.stack.size) #\(.number) draft=\(.draft) \(.title)"'
done
```

## Gotchas

- **A correct base chain is not a stack.** Always check `.stack` after creating a PR.
- **`gh` must be recent enough for the extension.** `gh extension install github/gh-stack`
  failing or `gh stack` reporting an unknown command means the CLI needs upgrading
  (`brew upgrade gh`).
- **The bottom PR drifts from `main`.** A long-lived stack's base commit ages; `gh stack sync`
  fast-forwards trunk and cascades, and `gh stack view` marks branches needing a rebase with `⚠`.
- **Force-pushes invalidate in-progress reviews.** Cascade-rebasing rewrites every branch above
  the change, so batch review rounds rather than rebasing after each comment.
- **Keep each layer independently green.** A reviewer may check out any single PR; a layer that
  only passes once a later one lands is mis-split.
