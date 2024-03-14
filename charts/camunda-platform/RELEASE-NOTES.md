The changelog is automatically generated using [git-chglog](https://github.com/git-chglog/git-chglog)
and it follows [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) format.


<a name="camunda-platform-8.3.10"></a>
## [camunda-platform-8.3.10](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.3.9...camunda-platform-8.3.10) (2024-03-12)

### Fix

* multiregion: add missing newline in start script ([#1436](https://github.com/camunda/camunda-platform-helm/pull/1436))

### Verification

To verify integrity of the Helm chart using [Cosign](https://docs.sigstore.dev/signing/quickstart/):

```shell
cosign verify-blob camunda-platform-8.3.10.tgz \
  --bundle camunda-platform-8.3.10.cosign.bundle \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  --certificate-identity "https://github.com/_GITHUB_WORKFLOW_REF_"
```
