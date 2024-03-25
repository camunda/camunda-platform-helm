**Before opening the PR:**

- [x] Run `make release.chores`
- [x] Create a PR with the printed URL
- [x] [Trigger Renovate](https://developer.mend.io/github/camunda/camunda-platform-helm) to ensure that all deps are updated (to avoid any delayed sync from Renovate service).

**After opening the PR:**

- [ ] Take a final look at the release commits (version bump and release notes)
- [ ] Make sure that all checks in the PR passed
- [ ] If everything in place, add `release` label to the PR
- [ ] Follow up the release workflow and make sure it succeeded (double-check the [releases page](https://github.com/camunda/camunda-platform-helm/releases))
- [ ] Ensure that all fixed [issues with support label](https://github.com/camunda/camunda-platform-helm/issues?q=is%3Aissue+label%3Asupport+is%3Aclosed) has the release version like `version:10.0.0` (this will be automated later).

If the release is a minor version bump (e.g. from 8.2.x to 8.3.0):
- [ ] Make sure the regression test matrix is updated: located at [.github/workflows/test-regression.yaml](https://github.com/camunda/camunda-platform-helm/blob/main/.github/workflows/test-regression.yaml#L33-L36)
