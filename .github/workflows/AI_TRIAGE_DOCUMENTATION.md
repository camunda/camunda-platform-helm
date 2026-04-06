# AI-Powered Bug & Task Triage Documentation

## Overview

This document describes the AI-powered triage automation system implemented for the `camunda-platform-helm` repository. The system automatically triages new and untriaged issues using Glean Chat API (RAG) and manages issue urgency based on severity and likelihood labels.

## Components

### 1. AI Triage Workflow (`ai-triage-issue.yml`)

Automatically triages issues using AI-powered analysis.

**Triggers:**
- `issues: [opened]` - Automatically when new issues are created
- `schedule: '0 10 * * *'` - Daily at 10:00 AM UTC
- `workflow_dispatch` - Manual trigger with optional parameters

**Process:**
1. Fetches untriaged issues from Project #33 (Status=📥 Inbox, no TRIAGE:COMPLETED label)
2. Calls Glean Chat API with issue content
3. Receives AI triage assessment (severity, likelihood, confidence, rationale)
4. Skips issues with:
   - Confidence < 0.7
   - `triage:manual` label
   - Severity = unknown
5. Applies labels: `severity/*`, `likelihood/*`, `TRIAGE:COMPLETED`
6. Posts structured triage assessment comment
7. Moves issue from Inbox → Backlog status
8. Triggers urgency calculation (via assign-urgency-to-issue.yml)

**Manual Usage:**

```bash
# Triage all untriaged issues
gh workflow run ai-triage-issue.yml

# Triage a specific issue
gh workflow run ai-triage-issue.yml -f issue_number=123

# Dry run (analyze but don't apply changes)
gh workflow run ai-triage-issue.yml -f issue_number=123 -f dry_run=true
```

### 2. Urgency Calculator Workflow (`assign-urgency-to-issue.yml`)

Automatically calculates and updates the Urgency field based on severity/likelihood labels.

**Triggers:**
- `issues: [opened, labeled, unlabeled, reopened, transferred]`
- `workflow_dispatch` - Manual trigger

**Urgency Matrix (Severity × Likelihood):**

| Severity / Likelihood | Low | Mid | High |
|---|---|---|---|
| **Low** | someday | someday | someday |
| **Mid** | someday | planned | next |
| **High** | planned | next | next |
| **Critical** | immediate | immediate | immediate |

**Alternative Matrix (Impact × When):**

| Impact / When | Later | Soon | Now |
|---|---|---|---|
| **Low** | someday | planned | planned |
| **Medium** | planned | next | next |
| **High** | planned | next | immediate |

**Manual Usage:**

```bash
# Calculate urgency for a specific issue
gh workflow run assign-urgency-to-issue.yml -f issue_number=123

# Dry run
gh workflow run assign-urgency-to-issue.yml -f issue_number=123 -f dry_run=true

# Target a different project
gh workflow run assign-urgency-to-issue.yml -f issue_number=123 -f project_id=33
```

### 3. Bulk Backfill Script (`assign-urgency-to-issues.sh`)

Batch processes multiple issues to calculate/update urgency.

**Usage:**

```bash
# Process first 20 issues in project 33
./.github/workflows/assign-urgency-to-issues.sh

# Process 50 issues
./.github/workflows/assign-urgency-to-issues.sh 33 main 50

# Only process issues without urgency assigned
./.github/workflows/assign-urgency-to-issues.sh 33 main 50 true

# Dry run
./.github/workflows/assign-urgency-to-issues.sh 33 main 50 true true
```

**Parameters:**
1. `PROJECT_ID` - Project number (default: 33)
2. `BRANCH` - Branch to trigger workflow on (default: main)
3. `LIMIT` - Number of issues to process (default: 20)
4. `SKIP_ASSIGNED` - Skip issues with urgency already set (default: false)
5. `DRY_RUN` - Don't trigger workflows, just show what would be done (default: false)

## Configuration

### Environment Variables

All workflows use these configurations:

- `PROJECT_NUMBER`: `33` (camunda-platform-helm project)
- `CONFIDENCE_THRESHOLD`: `0.7` (minimum AI confidence to auto-triage)
- `URGENCY_FIELD_NAME`: `Urgency`

### Authentication

Workflows use the existing Vault/GitHub App pattern:

```yaml
- name: Import Vault secrets
  uses: hashicorp/vault-action@4c06c5ccf5c0761b6029f56cfb1dcf5565918a3b # v3.4.0
  with:
    url: ${{ secrets.VAULT_ADDR }}
    method: approle
    roleId: ${{ secrets.VAULT_ROLE_ID }}
    secretId: ${{ secrets.VAULT_SECRET_ID }}
    secrets: |
      secret/data/products/distribution/ci GH_APP_ID_DISTRO_CI;
      secret/data/products/distribution/ci GH_APP_PRIVATE_KEY_DISTRO_CI;
      secret/data/products/distribution/ci GLEAN_API_TOKEN;
```

**Required Vault Secrets** (at `secret/data/products/distribution/ci`):
- `GH_APP_ID_DISTRO_CI` ✅ (already exists)
- `GH_APP_PRIVATE_KEY_DISTRO_CI` ✅ (already exists)
- `GLEAN_API_TOKEN` ⚠️ (needs to be added - see Prerequisites)

## Prerequisites

### 1. Glean API Token

**Status:** ⚠️ Pending

Request a Glean API token via [Freshservice general request](https://camunda.freshservice.com) with:
- Scopes: **CHAT** and **SEARCH**
- Endpoint: `camunda-be.glean.com`

Once received, add to Vault:
```bash
vault kv put secret/data/products/distribution/ci GLEAN_API_TOKEN=<token>
```

### 2. Repository Labels

The following labels must exist in the repository:

**Severity labels:**
- `severity/low`
- `severity/mid`
- `severity/high`
- `severity/critical`
- `severity/unknown`

**Likelihood labels:**
- `likelihood/low`
- `likelihood/mid`
- `likelihood/high`

**Impact labels (alternative):**
- `impact/low`
- `impact/medium`
- `impact/high`

**When labels (alternative):**
- `when/later`
- `when/soon`
- `when/now`

**Control labels:**
- `TRIAGE:COMPLETED` - Marks issue as triaged
- `triage:manual` - Skips AI triage, requires human review
- `BLOCKER/INFO` - Applied when missing information detected
- `urgency-locked` - Prevents automatic urgency updates

### 3. Project Fields

Project #33 must have:

**Status field:**
- Type: Single select
- Options: `📥 Inbox`, `Backlog`, (and others)

**Urgency field:**
- Name: `Urgency`
- Type: Single select
- Options: `immediate`, `next`, `planned`, `someday`

### 4. Glean Documentation Indexing

Verify these Confluence pages are indexed by Glean:
- [Distro - Bug Triage](https://confluence.camunda.com/spaces/HAN/pages/347898339/Distro+-+Bug+Triage)
- [Distro - Task / Toil / Non-Feature Triage](https://confluence.camunda.com/spaces/HAN/pages/347898345/Distro+-+Task+Toil+Non-Feature+Triage)
- [SM Platform - Team Boundaries](https://confluence.camunda.com/spaces/HAN/pages/347898341/SM+Platform+-+Team+Boundaries)

## Workflow Outputs

### AI Triage Comment Format

When an issue is triaged, the AI posts a comment like this:

```markdown
## 🧭 Triage Assessment

| Field | Value |
|-------|-------|
| Work type | **BUG** |
| Severity | ⚠️ **Mid** |
| Likelihood | 🔥 **High** |
| Confidence | **0.92** |

### 📝 Rationale

The issue describes a production deployment failure affecting multiple users.
According to the Distro Bug Triage guidelines, this qualifies as mid severity
(noticeable production impact, no data loss) with high likelihood (recurring issue).

### 🔍 Similar Issues

#456 - Similar Helm chart deployment error
#789 - Related ingress configuration issue

---
🤖 This issue was automatically triaged by an AI-based system.
✋ If you believe this assessment is incorrect, update the labels or add the `triage:manual` label for human review.
```

## Special Labels

### `triage:manual`

Add this label to an issue to:
- Skip AI triage completely
- Require human review
- Useful for complex or sensitive issues

### `urgency-locked`

Add this label to an issue to:
- Prevent automatic urgency updates
- Maintain manually-set urgency value
- Still shows calculated urgency but doesn't apply it

## Troubleshooting

### Issue Not Being Triaged

**Check:**
1. Is the issue in Project #33?
2. Is the issue status set to "📥 Inbox"?
3. Does the issue have the `triage:manual` label?
4. Does the issue already have the `TRIAGE:COMPLETED` label?

### Low Confidence (< 0.7)

If the AI confidence is too low, the issue won't be auto-triaged. This happens when:
- Issue description is vague or incomplete
- Similar issues have conflicting severity/likelihood
- Issue doesn't clearly fit triage guidelines

**Solution:** Add the `triage:manual` label for human review.

### Urgency Not Updating

**Check:**
1. Does the issue have both `severity/*` and `likelihood/*` labels (or `impact/*` and `when/*`)?
2. Is the issue in Project #33?
3. Does the issue have the `urgency-locked` label?
4. Does the Project #33 have the "Urgency" field configured?

### Glean API Errors

**Check:**
1. Is `GLEAN_API_TOKEN` set in Vault?
2. Does the token have CHAT and SEARCH scopes?
3. Are the Confluence triage pages indexed by Glean?

## Testing

### Dry Run Mode

Both workflows support dry-run mode for safe testing:

```bash
# Test AI triage without making changes
gh workflow run ai-triage-issue.yml -f issue_number=123 -f dry_run=true

# Test urgency calculation without updating
gh workflow run assign-urgency-to-issue.yml -f issue_number=123 -f dry_run=true

# Test bulk backfill without triggering workflows
./.github/workflows/assign-urgency-to-issues.sh 33 main 10 false true
```

### Single Issue Mode

Test workflows on a specific issue:

```bash
# Triage a single issue
gh workflow run ai-triage-issue.yml -f issue_number=123

# Calculate urgency for a single issue
gh workflow run assign-urgency-to-issue.yml -f issue_number=123
```

## Implementation References

Based on:
- [camunda/camunda assign-urgency-to-issue.yml](https://github.com/camunda/camunda/blob/main/.github/workflows/assign-urgency-to-issue.yml)
- [camunda/camunda assign-urgency-to-issues.sh](https://github.com/camunda/camunda/blob/main/.github/workflows/assign-urgency-to-issues.sh)
- [Glean Chat API connector template](https://github.com/camunda/eaat/blob/main/process-applications/ticket-genie/src/main/resources/camunda/.camunda/element-templates/Glean%20-%20Chat.json)

## Acceptance Criteria Status

- ✅ `ai-triage-issue.yml` runs daily at 10 AM UTC, on `issues: [opened]`, and on manual dispatch
- ✅ Fetches untriaged issues from Project #33 (Status=Inbox, no TRIAGE:COMPLETED label)
- ✅ Calls Glean Chat API with issue content (⚠️ pending token)
- ✅ Skips issues with confidence < 0.7 or `triage:manual` label
- ✅ Applies `severity/*` and `likelihood/*` labels based on AI assessment
- ✅ Posts a `🧭 Triage Assessment` comment with rationale
- ✅ Moves issue status from Inbox → Backlog
- ✅ `assign-urgency-to-issue.yml` reacts to label changes and sets Urgency field
- ✅ Both workflows use existing Vault/GitHub App auth pattern
- ✅ Manual dispatch supports single-issue mode and dry-run mode
- ✅ Bulk backfill script available for existing issues
- ✅ Handles both bug triage (severity/likelihood) and task/toil triage rules

**Pending:**
- ⚠️ Glean API token needs to be added to Vault
- ⚠️ Labels need to be created in repository
- ⚠️ Project #33 fields need to be verified
- ⚠️ Glean Confluence indexing needs to be verified
