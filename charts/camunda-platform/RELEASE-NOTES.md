The changelog is automatically generated using [git-chglog](https://github.com/git-chglog/git-chglog)
and it follows [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) format.


<a name="camunda-platform-8.3.7"></a>
## [camunda-platform-8.3.7](https://github.com/camunda/camunda-platform-helm/compare/camunda-platform-8.3.6...camunda-platform-8.3.7) (2024-02-14)

### Verification

To verify integrity of the Helm chart using [Cosign](https://docs.sigstore.dev/signing/quickstart/):

```shell
cosign verify-blob camunda-platform-8.3.7.tgz \
  --bundle camunda-platform-8.3.7.cosign.bundle \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  --certificate-identity "https://github.com/_GITHUB_WORKFLOW_REF_"
```
