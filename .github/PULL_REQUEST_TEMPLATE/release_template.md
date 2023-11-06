**Before opening the PR:**

- [x] Run `make release.chores`
- [x] Create a PR with the printed URL

**After opening the PR:**

- [ ] Take a final look at the release commits (version bump and release notes)
- [ ] Make sure that all checks in the PR passed
- [ ] If everything in place, add `release` label to the PR
- [ ] Follow up the release workflow and make sure it succeeded (double-check the [releases page](https://github.com/camunda/camunda-platform-helm/releases))

If the release is a minor version bump:
- [ ] Make sure the regression test matrix is updated: located at [.github/workflows/test-regression.yaml](https://github.com/camunda/camunda-platform-helm/blob/main/.github/workflows/test-regression.yaml#L33-L36)
