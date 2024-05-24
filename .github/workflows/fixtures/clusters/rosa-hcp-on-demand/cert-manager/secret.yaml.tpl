# TODO: Replace it with secret managment solution.
# until that, you can use envsubst https://stackoverflow.com/a/56009991
# envsubst < secret.yaml.tpl > secret.yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: cert-manager-gcp-service-account
data:
  # Get it from distro-central cluster.
  credentials.json: "$CERT_MANAGER_GCP_SERVICE_ACCOUNT"
