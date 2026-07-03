#!/usr/bin/env bash
# Copyright 2026 Camunda Services GmbH
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# Plaintext-fallback regression check for TLS-claimed deployments.
#
# Inspects every container env var in every pod of the target namespace and
# fails if any URL points at a known datastore service over plain HTTP, or if
# any jdbc:postgresql:// URL omits sslmode=verify-ca or sslmode=verify-full.
# Note that ssl=true alone and sslmode=require are deliberately rejected:
# both disable hostname verification, which is the regression we want to
# catch.
#
# This catches accidental chart regressions where a default plaintext URL
# silently re-enters a TLS-claimed deployment — the case the epic
# (product-hub#3520) explicitly cites as something the baseline must
# prevent. Run it after deploying any TLS-enabled scenario.
#
# Usage:
#   scripts/check-no-plaintext-datastore.sh \
#     --namespace <ns> \
#     [--kube-context <ctx>]
#
# Exit codes:
#   0 - clean, no plaintext datastore references found
#   1 - one or more violations (printed to stderr)
#   2 - usage / runtime error

set -euo pipefail

# ---------------------------------------------------------------------------
# Args
# ---------------------------------------------------------------------------
NAMESPACE=""
KUBE_CONTEXT=""

usage() {
  cat <<EOF
Usage: $0 --namespace <ns> [--kube-context <ctx>]

Checks pods in <ns> for plaintext datastore URLs that would indicate a
TLS-claimed deployment has silently regressed. Service-name patterns checked:
  opensearch, elasticsearch, postgres, postgresql (and their integration-* variants)

Exits 0 if clean, 1 if any plaintext URL found, 2 on usage/runtime error.
EOF
}

while (( "$#" )); do
  case "$1" in
    -n|--namespace)
      if (( $# < 2 )); then
        echo "ERROR: --namespace requires a value" >&2
        usage >&2
        exit 2
      fi
      NAMESPACE="$2"; shift 2 ;;
    --kube-context)
      if (( $# < 2 )); then
        echo "ERROR: --kube-context requires a value" >&2
        usage >&2
        exit 2
      fi
      KUBE_CONTEXT="$2"; shift 2 ;;
    -h|--help)
      usage; exit 0 ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 2 ;;
  esac
done

if [[ -z "${NAMESPACE}" ]]; then
  echo "ERROR: --namespace is required" >&2
  usage >&2
  exit 2
fi

KCTL_ARGS=(-n "${NAMESPACE}")
if [[ -n "${KUBE_CONTEXT}" ]]; then
  KCTL_ARGS+=(--context "${KUBE_CONTEXT}")
fi

# ---------------------------------------------------------------------------
# Patterns
# ---------------------------------------------------------------------------

# Datastore hostnames we expect to be TLS-protected. Matches common naming:
#   opensearch-master, integration-elasticsearch, postgres-headless, etc.
DATASTORE_NAMES='elasticsearch-master|elasticsearch|opensearch-master|opensearch|postgres|postgresql'
BITNAMI_SUFFIXES='-headless|-coordinating-only|-data|-master|-client'
PLAINTEXT_HTTP_RE="http://([a-z0-9-]*-)?(${DATASTORE_NAMES})(${BITNAMI_SUFFIXES})?([./:]|$)"

# Matches any jdbc:postgresql:// URL (stops at whitespace or quote).
JDBC_PG_RE='jdbc:postgresql://[^" ]*'

# ---------------------------------------------------------------------------
# Functions
# ---------------------------------------------------------------------------

# Extract all env vars from a pod as lines of: pod|container|name|value
# Returns 1 if kubectl/jq fails.
get_pod_env_vars() {
  local pod="$1"
  kubectl "${KCTL_ARGS[@]}" get pod "${pod}" -o json 2>/tmp/kubectl_err \
    | jq -r --arg pod "${pod}" '
        .spec.containers[], .spec.initContainers[]
        | .name as $container
        | .env[]?
        | select(.value != null and .value != "")
        | [$pod, $container, .name, .value] | join("|")
      '
}

# Check if a URL is plaintext HTTP to a datastore service.
is_plaintext_datastore_url() {
  echo "$1" | grep -qiE "${PLAINTEXT_HTTP_RE}"
}

# Check if a JDBC URL has proper TLS (sslmode=verify-ca or verify-full).
jdbc_url_has_tls() {
  echo "$1" | grep -qE '[?&]sslmode=verify-(ca|full)'
}

# Split compound JDBC URLs (joined by &jdbc:postgresql://) and check each one.
# Appends violations to VIOLATIONS array.
check_jdbc_urls() {
  local pod_name="$1" container="$2" env_name="$3" env_value="$4"

  # Split on &jdbc:postgresql:// boundary so each URL is checked independently.
  local split_value
  split_value=$(echo "${env_value}" | sed 's|&jdbc:postgresql://|\n\&jdbc:postgresql://|g; s|^&||')

  local url
  for url in $(echo "${split_value}" | grep -oE "${JDBC_PG_RE}"); do
    if ! jdbc_url_has_tls "${url}"; then
      VIOLATIONS+=("INSECURE-JDBC	${pod_name}/${container}	${env_name}=${url}")
    fi
  done
}

# ---------------------------------------------------------------------------
# Inspect
# ---------------------------------------------------------------------------
PODS=$(kubectl "${KCTL_ARGS[@]}" get pods -o jsonpath='{.items[*].metadata.name}' || true)
if [[ -z "${PODS}" ]]; then
  echo "ERROR: no pods found in namespace ${NAMESPACE}" >&2
  exit 2
fi

VIOLATIONS=()
SKIPPED_PODS=0

for POD in ${PODS}; do
  if ! RAW=$(get_pod_env_vars "${POD}"); then
    echo "WARNING: failed to inspect pod ${POD} (kubectl/jq error)" >&2
    SKIPPED_PODS=$((SKIPPED_PODS + 1))
    continue
  fi

  [[ -z "${RAW}" ]] && continue

  while IFS='|' read -r pod_name container env_name env_value; do
    if is_plaintext_datastore_url "${env_value}"; then
      VIOLATIONS+=("PLAINTEXT-HTTP	${pod_name}/${container}	${env_name}=${env_value}")
    fi

    if echo "${env_value}" | grep -qE "${JDBC_PG_RE}"; then
      check_jdbc_urls "${pod_name}" "${container}" "${env_name}" "${env_value}"
    fi
  done <<< "${RAW}"
done

# ---------------------------------------------------------------------------
# Report
# ---------------------------------------------------------------------------
if [[ ${#VIOLATIONS[@]} -eq 0 && ${SKIPPED_PODS} -gt 0 ]]; then
  echo "[no-plaintext-check] INCONCLUSIVE: no violations found but ${SKIPPED_PODS} pod(s) could not be inspected" >&2
  exit 2
fi

if [[ ${#VIOLATIONS[@]} -eq 0 ]]; then
  echo "[no-plaintext-check] PASS: no plaintext datastore URLs found in ${NAMESPACE}"
  exit 0
fi

echo "[no-plaintext-check] FAIL: ${#VIOLATIONS[@]} plaintext datastore reference(s) in ${NAMESPACE}:" >&2
if command -v column &>/dev/null; then
  printf '%s\n' "${VIOLATIONS[@]}" | column -t -s $'\t' >&2
else
  printf '%s\n' "${VIOLATIONS[@]}" >&2
fi
if [[ ${SKIPPED_PODS} -gt 0 ]]; then
  echo "WARNING: additionally ${SKIPPED_PODS} pod(s) could not be inspected" >&2
  exit 2
fi
exit 1
