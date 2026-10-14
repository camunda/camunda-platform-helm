# Issue triage

Shared issue-author classification, Glean assessment, project routing, and Slack
intake for the Distribution workflows. Implements [#7568](https://github.com/camunda/camunda-platform-helm/issues/7568).

## Policy

| Creator | Assessment | Priority | Discussion / Slack | Status |
|---|---|---|---|---|
| `camunda/distribution` or `camunda/infrastructure-experience` member | AI suggestion | Fill missing severity/likelihood labels; calculate project Urgency | Actions summary only | New issues go to Backlog; preserve deliberate status changes; no automatic Ready promotion |
| Other human or bot | Existing visible assessment | AI labels and project Urgency | Triage comment and Slack intake | Existing external routing, including PM feature-request assignment |

Membership is resolved from both paginated team member lists (including child-team
members returned by GitHub) and cached within a command invocation. The creator,
not the event actor or repository/organization association, determines the policy.
Bots remain external. A missing/inaccessible/empty team fails classification before
side effects. Membership is evaluated at processing time, including manual runs.

Internal suggestions preserve existing severity, likelihood, and kind labels.
Existing impact/when labels continue to participate in the urgency calculation.
`urgency-locked` prevents urgency changes and Ready promotion. Internal assessments
below 0.7 confidence, unknown severity, and failures are recorded for review in the
Actions summary without filling missing priority labels or posting issue comments.

## Workflows

- `triage-ai-issue.yml`: select candidates, then process one issue per matrix job.
  All entry points share an issue-number concurrency group. Assessment errors are
  collected so external Slack intake still runs and the job then fails visibly.
- `add-new-issue.yaml`: classify before project insertion; on `opened`, route
  internal issues to Backlog only from an empty/Inbox status. Reconcile urgency
  through the reusable urgency workflow after project insertion. Together with
  label events, this handles assessment finishing before or after insertion.
- `triage-assign-urgency-to-issue.yml`: use the shared classification before
  applying urgency; only external issues may be promoted to Ready.

Delivery-state label events are ignored by these workflows. Other repeated events
are safe for intake delivery because the serialized worker refreshes labels before
sending. The single-issue manual input does not bypass author policy.

## Commands

Run from the repository root with `GH_TOKEN` and `GITHUB_REPOSITORY` set:

```sh
ISSUE_NUMBER=7568 make run.issue-triage command=classify
ISSUE_NUMBER=7568 make run.issue-triage command=route
ISSUE_NUMBER=7568 make run.issue-triage command=triage
make run.issue-triage command=select
make test.issue-triage
```

`route` only updates internal issues. `triage` also needs `GLEAN_API_TOKEN` and,
for external intake, `SLACK_WEBHOOK`. `PROJECT_NUMBER` defaults to 33.
`GITHUB_API_URL`, `GITHUB_OUTPUT`, and `GITHUB_STEP_SUMMARY` use the Actions
environment. Workflow secrets come from Vault and are scoped to consuming steps.

Bulk selection paginates open repository issues and their project memberships.
It includes unassessed Inbox issues, unassessed internal Backlog issues, external
manual-triage Inbox issues, and outstanding external delivery retries. Previously
assessed external Backlog issues without a retry marker are not retroactively
broadcast. More than 256 candidates fails with a request to process individual
issues first (the Actions matrix limit).

## Slack delivery and recovery

Slack intake runs after assessment, even for low confidence, assessment errors,
manual triage, or PM routing. Confirmed delivery is recorded with
`triage:slack-notified`. `triage:slack-retry` records an outstanding intake attempt;
`triage:slack-pending` is written immediately before the webhook call. Labels are
created automatically. A confirmed `200 ok` records the receipt before clearing
pending/retry state. An explicit HTTP 4xx rejection clears pending so dispatching
the workflow again can retry. Delivery errors fail the Actions job.

Incoming webhooks have no idempotency key or transactional receipt shared with
GitHub. Timeouts, HTTP 5xx, interrupted runs, and a delivered message whose receipt
could not be written are ambiguous. Pending deliveries require reconciliation;
automatic retries could duplicate a message:

1. Inspect the issue's workflow run and `#team-distribution-github`.
2. If the message arrived, add `triage:slack-notified`, then remove
   `triage:slack-pending` and `triage:slack-retry`.
3. If it did not arrive, remove `triage:slack-pending` and manually dispatch
   `triage-ai-issue.yml` with that issue number.

Keep these operational labels restricted to maintainers. Manual removal of a
confirmed receipt permits another notification. Standalone `triage` invocations
must not overlap for the same issue; serialization is supplied by Actions.

## Installation and rollout

The `distro-ci` App installation needs organization **Members: read** and
**Projects: write**, and repository **Issues: write**. The selection job requests
read-only issue/project access. The App declares the required permissions; an
organization administrator must also approve them on the installation.

Before enabling production intake, validate classification with the installation
token, then verify one internal issue and one external issue through the workflows.
Use the test suite for rejected/ambiguous Slack deliveries rather than sending
test failures to the production channel. Check Actions summaries for internal
assessment failures requiring review.
