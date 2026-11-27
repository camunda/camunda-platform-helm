# The docs-sync PR comment

CI and the local run share state through one comment on the Helm PR. Hidden
HTML markers carry the state, so the comment stays readable while the workflow
and the skill can still parse it.

## Markers

| Marker | Written by | Meaning |
|---|---|---|
| `<!-- docs-sync:comment -->` | CI and local | Identifies the comment. Exactly one per PR. |
| `<!-- docs-sync:impact=yes -->` / `=no` | CI and local | Whether the PR needs docs changes. First line of the body. |
| `<!-- docs-sync:analyzed-head=<sha> -->` | CI and local | Helm PR head commit the analysis was made for. |
| `<!-- docs-sync:docs-pr=<url> -->` | Local only | The camunda-docs PR opened for this Helm PR. |
| `<!-- docs-sync:synced-head=<sha> -->` | Local only | Helm PR head commit the docs PR was last brought in line with. |

CI must carry `docs-pr` and `synced-head` forward unchanged when it rewrites
the comment; the workflow's upsert step does that, so CI-generated bodies
should not include them.

## Body template

Keep it short: reviewers read it in the PR timeline. Use this structure.

```markdown
<!-- docs-sync:impact=yes -->
<!-- docs-sync:comment -->
<!-- docs-sync:analyzed-head=abc1234def -->
### 📚 Docs impact

This PR changes user-facing chart behavior, so [camunda-docs](https://github.com/camunda/camunda-docs) needs an update.

| Chart | Docs folder | Changes |
|---|---|---|
| 8.11 | `docs/` (Next) | 1 default changed (breaking) |
| 8.10 | `versioned_docs/version-8.10` | 1 default changed (breaking) |
| 8.7 | — | Not maintained in camunda-docs, skipped |

**What changes for users**

- `optimize.database.elasticsearch.url.host` now defaults to empty instead of `{{ .Release.Name }}-elasticsearch`. Users relying on the old default must set the host explicitly. **Breaking.**

**Proposed docs edits**

- `self-managed/deployment/helm/configure/database/optimize/using-external-elasticsearch.md`: state that the host has no default and must be set.
- `reference/announcements-release-notes/8100/8100-announcements.md`: breaking-change entry.
- Upgrade guide for 8.11 does not exist yet; announcement only.

**Next step:** run `/docs-sync` in Claude Code on this branch to open a draft docs PR.
```

When a docs PR already exists, replace the last line with the sync status:

- In sync: `Docs PR: <url> (in sync with <short-sha>)`
- Behind: `Docs PR: <url> is **behind** this PR: new docs-relevant changes since <synced short-sha>. Run /docs-sync to update it.`

Only call the docs PR "behind" when the commits after `synced-head` change
something in the impact list. A rebase or a test-only commit does not count.

## No impact

```markdown
<!-- docs-sync:impact=no -->
<!-- docs-sync:comment -->
<!-- docs-sync:analyzed-head=abc1234def -->
### 📚 Docs impact

No docs changes needed: <one-line reason, e.g. "image tag bumps only">.
```

The workflow only posts a no-impact comment when one already exists (the PR
used to have impact). It stays silent on PRs that never had any.
