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
# any jdbc:postgresql:// URL omits TLS (sslmode=verify-* or ssl=true).
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
  opensearch, elasticsearch, postgres-tls (and their integration-* variants)

Exits 0 if clean, 1 if any plaintext URL found, 2 on usage/runtime error.
EOF
}

while (( "$#" )); do
  case "$1" in
    -n|--namespace)
      NAMESPACE="$2"; shift 2 ;;
    --kube-context)
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
# HTTP plaintext to known datastore service names. The trailing colon-or-slash
# is intentional: it avoids matching paths like "http-elasticsearch" or random
# substrings; the URL boundary makes the match intentional.
PLAINTEXT_HTTP_RE='http://([a-z0-9-]*-)?(elasticsearch|opensearch|postgres|postgresql|opensearch-master|elasticsearch-master)([./:]|$)'

# JDBC PostgreSQL URLs that are missing a TLS directive. Postgres JDBC
# accepts either ?ssl=true or ?sslmode=require|verify-ca|verify-full. We
# require verify-ca or verify-full as the only meaningful TLS settings;
# ssl=true alone disables hostname verification on Postgres JDBC, which is
# the regression we want to catch.
JDBC_PG_RE='jdbc:postgresql://[^"]*'

# ---------------------------------------------------------------------------
# Inspect
# ---------------------------------------------------------------------------
PODS=$(kubectl "${KCTL_ARGS[@]}" get pods -o jsonpath='{.items[*].metadata.name}' || true)
if [[ -z "${PODS}" ]]; then
  echo "ERROR: no pods found in namespace ${NAMESPACE}" >&2
  exit 2
fi

VIOLATIONS=()

for POD in ${PODS}; do
  # Each container's env (.spec.containers[*].env[*]) and initContainer envs.
  # Output one line per env var: <pod>|<container>|<env-name>|<env-value>
  RAW=$(kubectl "${KCTL_ARGS[@]}" get pod "${POD}" -o json 2>/dev/null \
    | jq -r --arg p "${POD}" '
        (.spec.containers // []) + (.spec.initContainers // [])
        | .[] as $c
        | ($c.env // [])[]
        | select(.value != null and .value != "")
        | "\($p)|\($c.name)|\(.name)|\(.value)"
      ' 2>/dev/null) || true

  if [[ -z "${RAW}" ]]; then
    continue
  fi

  while IFS='|' read -r pod_name container env_name env_value; do
    # 1. Plaintext HTTP to datastore service names.
    if echo "${env_value}" | grep -qiE "${PLAINTEXT_HTTP_RE}"; then
      VIOLATIONS+=("PLAINTEXT-HTTP	${pod_name}/${container}	${env_name}=${env_value}")
    fi

    # 2. Postgres JDBC URLs without sslmode=verify-* (verify-ca / verify-full).
    #    Match each occurrence; an env var can contain multiple URLs.
    if echo "${env_value}" | grep -qE "${JDBC_PG_RE}"; then
      for url in $(echo "${env_value}" | grep -oE "${JDBC_PG_RE}"); do
        if ! echo "${url}" | grep -qE 'sslmode=verify-(ca|full)'; then
          VIOLATIONS+=("INSECURE-JDBC	${pod_name}/${container}	${env_name}=${url}")
        fi
      done
    fi
  done <<< "${RAW}"
done

# ---------------------------------------------------------------------------
# Report
# ---------------------------------------------------------------------------
if [[ ${#VIOLATIONS[@]} -eq 0 ]]; then
  echo "[no-plaintext-check] PASS: no plaintext datastore URLs found in ${NAMESPACE}"
  exit 0
fi

echo "[no-plaintext-check] FAIL: ${#VIOLATIONS[@]} plaintext datastore reference(s) in ${NAMESPACE}:" >&2
printf '%s\n' "${VIOLATIONS[@]}" | column -t -s $'\t' >&2
exit 1
