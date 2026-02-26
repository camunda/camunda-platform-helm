#!/bin/bash

set -euo pipefail

# Deploy Elasticsearch via Helm and create an ExternalSecret in the same namespace.
# Defaults target Bitnami Elasticsearch chart v21.6.3 and the repo's infra values.

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

RELEASE_NAME="elasticsearch"
CHART="bitnami/elasticsearch"
CHART_VERSION="21.6.3"
VALUES_FILE=""
NAMESPACE=""
POOL_INDEX=""

# ExternalSecret defaults
EXTERNAL_SECRET_FILE="$REPO_ROOT/.github/config/external-secret/external-secret-infra.yaml"
EXTERNAL_SECRET_CERT_FILE="$REPO_ROOT/.github/config/external-secret/external-secret-certificates.yaml"

# Helm behavior defaults
HELM_TIMEOUT="10m0s"
ATOMIC=true
SKIP_EXTERNAL_SECRET=false

# Basic logging helpers
die() { echo "[error] $*" >&2; exit 1; }
check_cmd() { command -v "$1" >/dev/null 2>&1 || die "Required command '$1' not found in PATH"; }
trap 'die "Script failed (line $LINENO)"' ERR

usage() {
  cat <<EOF
Usage: $0 [options]

Options:
  --release RELEASE_NAME           Helm release name (default: ${RELEASE_NAME})
  --chart CHART                    Helm chart (default: ${CHART})
  --version CHART_VERSION          Helm chart version (default: ${CHART_VERSION})
  --values PATH                    Values file (default: infra/elasticsearch/${CHART_VERSION}/values.yaml)
  --namespace NAMESPACE            Kubernetes namespace to deploy into (default derived from --version)
  --external-secret-file PATH      Path to ExternalSecret YAML to apply (default: ${EXTERNAL_SECRET_FILE})
  --skip-external-secret           Do not apply ExternalSecret
  --timeout DURATION               Helm wait timeout (default: ${HELM_TIMEOUT})
  --atomic | --no-atomic           Helm atomic upgrade (default: --atomic)
  --pool-index N                   ES pool index (e.g. 0 or 1); appends -pool-N to namespace & ingress
  --set KEY=VALUE                  Extra Helm --set (repeatable)
  --set-string KEY=VALUE           Extra Helm --set-string (repeatable)
  -h, --help                       Show this help

Examples:
  $0 --release es --version 21.6.3 \
     --namespace my-elasticsearch \
     --external-secret-file .github/config/external-secret/external-secret-credentials.yaml \
     --set security.existingSecret=integration-test-credentials
EOF
}

HELM_SET_ARGS=()
HELM_SET_STRING_ARGS=()

while [[ $# -gt 0 ]]; do
  case "$1" in
  --release)
    RELEASE_NAME="$2"
    shift 2
    ;;
  --chart)
    CHART="$2"
    shift 2
    ;;
  --version)
    CHART_VERSION="$2"
    shift 2
    ;;
  --values)
    VALUES_FILE="$2"
    shift 2
    ;;
  --namespace)
    NAMESPACE="$2"
    shift 2
    ;;
  --external-secret-file)
    EXTERNAL_SECRET_FILE="$2"
    shift 2
    ;;
  --skip-external-secret)
    SKIP_EXTERNAL_SECRET=true
    shift 1
    ;;
  --timeout)
    HELM_TIMEOUT="$2"
    shift 2
    ;;
  --atomic)
    ATOMIC=true
    shift 1
    ;;
  --no-atomic)
    ATOMIC=false
    shift 1
    ;;
  --set)
    HELM_SET_ARGS+=("--set" "$2")
    shift 2
    ;;
  --set-string)
    HELM_SET_STRING_ARGS+=("--set-string" "$2")
    shift 2
    ;;
  --pool-index)
    POOL_INDEX="$2"
    shift 2
    ;;
  -h | --help)
    usage
    exit 0
    ;;
  *)
    echo "Unknown option: $1" >&2
    usage
    exit 1
    ;;
  esac
done

# Derive namespace and default values file from chart version if not provided
CHART_VERSION_DASHED="$(echo "$CHART_VERSION" | tr '.' '-')"
if [[ -z "${NAMESPACE}" ]]; then
  NAMESPACE="distribution-elasticsearch-${CHART_VERSION_DASHED}"
  if [[ -n "${POOL_INDEX}" ]]; then
    NAMESPACE="${NAMESPACE}-pool-${POOL_INDEX}"
  fi
fi
if [[ -z "$VALUES_FILE" ]]; then
  VALUES_FILE="$REPO_ROOT/infra/elasticsearch/${CHART_VERSION}/values.yaml"
fi

# When a pool index is specified, override ingress hostnames so each pool gets
# its own DNS entry (ExternalDNS creates the record from the Ingress resource).
if [[ -n "${POOL_INDEX}" ]]; then
  HELM_SET_ARGS+=(
    --set "ingress.hostname=elasticsearch-${CHART_VERSION_DASHED}-pool-${POOL_INDEX}.ci.distro.ultrawombat.com"
    --set "ingress.extraTls[0].hosts[0]=elasticsearch-${CHART_VERSION_DASHED}-pool-${POOL_INDEX}.ci.distro.ultrawombat.com"
    --set "kibana.ingress.hostname=kibana-${CHART_VERSION_DASHED}-pool-${POOL_INDEX}.ci.distro.ultrawombat.com"
    --set "kibana.ingress.extraTls[0].hosts[0]=kibana-${CHART_VERSION_DASHED}-pool-${POOL_INDEX}.ci.distro.ultrawombat.com"
  )
fi

echo "[elasticsearch] Namespace: ${NAMESPACE}"
echo "[elasticsearch] Release:    ${RELEASE_NAME}"
echo "[elasticsearch] Chart:      ${CHART}@${CHART_VERSION}"
echo "[elasticsearch] Values:     ${VALUES_FILE}"
if [[ "${SKIP_EXTERNAL_SECRET}" == false ]]; then
  echo "[externalsecret] File:      ${EXTERNAL_SECRET_FILE}"
else
  echo "[externalsecret] Skipped"
fi
echo "[helm] Wait timeout:        ${HELM_TIMEOUT} (atomic=${ATOMIC})"

# Preflight
check_cmd kubectl
check_cmd helm
[[ -f "$VALUES_FILE" ]] || die "Values file not found: $VALUES_FILE"
if [[ "${SKIP_EXTERNAL_SECRET}" == false ]]; then
  [[ -f "$EXTERNAL_SECRET_FILE" ]] || die "ExternalSecret file not found: $EXTERNAL_SECRET_FILE"
  if ! kubectl get crd externalsecrets.external-secrets.io >/dev/null 2>&1; then
    echo "[warn] ExternalSecrets CRD not found; applying the ExternalSecret may fail" >&2
  fi
fi

# Ensure namespace exists
kubectl get namespace "$NAMESPACE" >/dev/null 2>&1 || kubectl create namespace "$NAMESPACE"

# Ensure Bitnami repo exists if using it
if [[ "$CHART" == bitnami/* ]]; then
  helm repo add bitnami https://charts.bitnami.com/bitnami >/dev/null 2>&1 || true
  helm repo update >/dev/null
fi

if [[ "${SKIP_EXTERNAL_SECRET}" == false ]]; then
  kubectl apply -n "$NAMESPACE" -f "$EXTERNAL_SECRET_FILE"
  kubectl apply -n "$NAMESPACE" -f "$EXTERNAL_SECRET_CERT_FILE"
  echo "ExternalSecret applied."
fi

# Install/upgrade Elasticsearch
[[ -n "$RELEASE_NAME" ]] || die "Release name is empty"
[[ -n "$CHART" ]] || die "Chart is empty"

HELM_FLAGS=(
  --namespace "$NAMESPACE"
  --version "$CHART_VERSION"
  -f "$VALUES_FILE"
  --wait
  --timeout "$HELM_TIMEOUT"
)
if [[ "${ATOMIC}" == true ]]; then
  HELM_FLAGS+=(--atomic)
fi

CMD=(helm upgrade --install "$RELEASE_NAME" "$CHART" "${HELM_FLAGS[@]}")
if (( ${#HELM_SET_ARGS[@]} > 0 )); then
  CMD+=("${HELM_SET_ARGS[@]}")
fi
if (( ${#HELM_SET_STRING_ARGS[@]} > 0 )); then
  CMD+=("${HELM_SET_STRING_ARGS[@]}")
fi

"${CMD[@]}"

echo "Elasticsearch Helm release applied."

# Create ConfigMap from the cleanup script so CronJobs can mount it.
# The ci-template-cleanup CronJob expects the script at /scripts/ci-cleanup-elasticsearch.sh.
CLEANUP_SCRIPT="$REPO_ROOT/scripts/ci-cleanup-elasticsearch.sh"
if [[ -f "$CLEANUP_SCRIPT" ]]; then
  echo "[configmap] Creating/updating ci-cleanup-script ConfigMap from: $CLEANUP_SCRIPT"
  kubectl create configmap ci-cleanup-script \
    --from-file=ci-cleanup-elasticsearch.sh="$CLEANUP_SCRIPT" \
    --namespace "$NAMESPACE" \
    --dry-run=client -o yaml | kubectl apply -f -
  echo "[configmap] ci-cleanup-script ConfigMap applied."
else
  echo "[configmap] WARN: $CLEANUP_SCRIPT not found — skipping ConfigMap creation."
fi

# Apply all CronJob manifests (*-cronjob.yaml) from the infra directory for this chart version.
# Each manifest contains its own namespace metadata and RBAC resources,
# so we use a plain 'kubectl apply -f' without a -n flag.
CRONJOB_DIR="$REPO_ROOT/infra/elasticsearch/${CHART_VERSION}"
cronjob_found=false
for CRONJOB_FILE in "${CRONJOB_DIR}"/*-cronjob.yaml; do
  [[ -f "$CRONJOB_FILE" ]] || continue
  cronjob_found=true
  echo "[cronjob] Applying CronJob from: $CRONJOB_FILE"
  if [[ -n "${POOL_INDEX}" ]]; then
    # Replace the base namespace with the pool-specific namespace in the manifest
    sed "s/distribution-elasticsearch-${CHART_VERSION_DASHED}/distribution-elasticsearch-${CHART_VERSION_DASHED}-pool-${POOL_INDEX}/g" \
      "$CRONJOB_FILE" | kubectl apply -f -
  else
    kubectl apply -f "$CRONJOB_FILE"
  fi
  echo "[cronjob] Applied: $(basename "$CRONJOB_FILE")"
done
if [[ "$cronjob_found" == false ]]; then
  echo "[cronjob] No CronJob manifests found in $CRONJOB_DIR — skipping."
fi
