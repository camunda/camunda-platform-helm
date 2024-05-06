### Release Info

{{ getenv "VERSION_MATRIX_RELEASE_INFO" }}

### Verification

To verify the integrity of the Helm chart using [Cosign](https://docs.sigstore.dev/signing/quickstart/):

```shell
cosign verify-blob {{ getenv "CHART_NAME_WITH_VERSION" }}.tgz \
  --bundle {{ getenv "CHART_NAME_WITH_VERSION" }}.cosign.bundle \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  --certificate-identity "https://github.com/{{ getenv "GITHUB_WORKFLOW_REF" }}"
```
