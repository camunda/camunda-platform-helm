The changelog is automatically generated using [git-chglog](https://github.com/git-chglog/git-chglog)
and it follows [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) format.


<a name="camunda-platform-8.3.4"></a>
## [camunda-platform-8.3.4](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.3.3...camunda-platform-8.3.4) (2023-12-12)

### Fix

*  removed hardcoded contextPaths while trimming redirectRootUrl ([#1118](https://github.com/camunda/camunda-platform-helm/issues/1118))

### Refactor

* show error message for optimize requirements ([#1132](https://github.com/camunda/camunda-platform-helm/issues/1132))
* mount tasklist-configmap volume on a new path ([#1101](https://github.com/camunda/camunda-platform-helm/issues/1101))
* enable tasklist user access restrictions ([#1093](https://github.com/camunda/camunda-platform-helm/issues/1093))

### Test

* re-enable prometheus tests ([#1107](https://github.com/camunda/camunda-platform-helm/issues/1107))

### Verification

To verify integrity of the Helm chart using [Cosign](https://docs.sigstore.dev/signing/quickstart/):

```shell
cosign verify-blob camunda-platform-8.3.4.tgz \
  --bundle camunda-platform-8.3.4.cosign.bundle \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  --certificate-identity "https://github.com/_GITHUB_WORKFLOW_REF_"
```
