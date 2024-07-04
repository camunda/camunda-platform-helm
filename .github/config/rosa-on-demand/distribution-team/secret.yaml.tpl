# Use envsubst https://stackoverflow.com/a/56009991
# envsubst < secret.yaml.tpl > secret.yaml
# This secret is templated in .github/workflows/test-integration-rosa.yaml#L228
---
apiVersion: v1
kind: Secret
metadata:
  name: external-clusters-access-secret-store-token
data:
  ca.crt: "$EXTERNAL_SECRET_STORE_SA_CA"
  service-ca.crt: "$EXTERNAL_SECRET_STORE_SA_SERVICE_CA"
  namespace: ZGlzdHJpYnV0aW9uLXRlYW0=
  # Get it from distro-central cluster.
  token: "$EXTERNAL_SECRET_STORE_SA_TOKEN"
