#!/bin/bash

set -euo pipefail

# Delete Keycloak Helm release and related resources.

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

RELEASE_NAME="keycloak"
CHART_VERSION="24.9.0"
NAMESPACE="keycloak-24-9-0"
EXTERNAL_SECRET_FILE="$REPO_ROOT/.github/config/external-secret/external-secret-infra.yaml"

HELM_TIMEOUT="10m0s"
SKIP_EXTERNAL_SECRET=false
PURGE_PVCS=false
DELETE_NAMESPACE=false

die() {
  echo "[error] $*" >&2
  exit 1
}
check_cmd() { command -v "$1" >/dev/null 2>&1 || die "Required command '$1' not found in PATH"; }
trap 'die "Script failed (line $LINENO)"' ERR

usage() {
  cat <<EOF
Usage: $0 [options]

Options:
  --release RELEASE_NAME           Helm release name (default: ${RELEASE_NAME})
  --version CHART_VERSION          Chart version to derive namespace (default: ${CHART_VERSION})
  --external-secret-file PATH      ExternalSecret YAML to delete (default: ${EXTERNAL_SECRET_FILE})
  --skip-external-secret           Do not delete ExternalSecret
  --purge-pvcs                     Delete PVCs labeled with app.kubernetes.io/instance=<release>
  --delete-namespace               Delete the namespace after uninstall
  --timeout DURATION               Helm uninstall wait timeout (default: ${HELM_TIMEOUT})
  -h, --help                       Show this help

Examples:
  $0 --release kc --version 24.9.0 --purge-pvcs --delete-namespace
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
  --release)
    RELEASE_NAME="$2"
    shift 2
    ;;
  --version)
    CHART_VERSION="$2"
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
  --purge-pvcs)
    PURGE_PVCS=true
    shift 1
    ;;
  --delete-namespace)
    DELETE_NAMESPACE=true
    shift 1
    ;;
  --timeout)
    HELM_TIMEOUT="$2"
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

# Derive namespace from chart version
NAMESPACE="keycloak-$(echo "$CHART_VERSION" | tr '.' '-')"

echo "[keycloak] Namespace: ${NAMESPACE}"
echo "[keycloak] Release:    ${RELEASE_NAME}"
if [[ "${SKIP_EXTERNAL_SECRET}" == false ]]; then
  echo "[externalsecret] File:      ${EXTERNAL_SECRET_FILE}"
else
  echo "[externalsecret] Skipped"
fi
echo "[cleanup] purge-pvcs=${PURGE_PVCS} delete-namespace=${DELETE_NAMESPACE}"

check_cmd kubectl
check_cmd helm

if [[ "${SKIP_EXTERNAL_SECRET}" == false ]]; then
  if [[ -f "$EXTERNAL_SECRET_FILE" ]]; then
    kubectl delete -n "$NAMESPACE" -f "$EXTERNAL_SECRET_FILE" --ignore-not-found
    echo "ExternalSecret deleted."
  else
    echo "[warn] ExternalSecret file not found: $EXTERNAL_SECRET_FILE" >&2
  fi
fi

if helm -n "$NAMESPACE" status "$RELEASE_NAME" >/dev/null 2>&1; then
  helm uninstall "$RELEASE_NAME" -n "$NAMESPACE" --wait --timeout "$HELM_TIMEOUT"
  echo "Helm release '$RELEASE_NAME' uninstalled."
else
  echo "[info] Helm release '$RELEASE_NAME' not found in namespace '$NAMESPACE'. Skipping uninstall."
fi

if [[ "${PURGE_PVCS}" == true ]]; then
  kubectl delete pvc -n "$NAMESPACE" -l app.kubernetes.io/instance="$RELEASE_NAME" --ignore-not-found || true
  echo "PVCs (if any) deleted."
fi

if [[ "${DELETE_NAMESPACE}" == true ]]; then
  kubectl delete namespace "$NAMESPACE" --ignore-not-found || true
  echo "Namespace '$NAMESPACE' deletion requested."
fi

echo "Keycloak cleanup complete."
