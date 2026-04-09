#!/usr/bin/env bash
# Harbor registry retry helpers for CI workflows.
# Source this file to get retry functions for transient Harbor 401 errors.
#
# Required env vars: HARBOR_REGISTRY_HOST (or HARBOR_REGISTRY),
#   HARBOR_REGISTRY_USER, HARBOR_REGISTRY_PASSWORD.
#
# Usage:
#   source scripts/harbor-retry.sh
#   harbor_retry "helm push" helm push chart.tgz oci://registry/project
#   harbor_login   # login with retry

HARBOR_RETRY_MAX=${HARBOR_RETRY_MAX:-3}
HARBOR_RETRY_DELAY=${HARBOR_RETRY_DELAY:-10}

# Resolve which env var holds the registry hostname.
_harbor_host() {
  echo "${HARBOR_REGISTRY_HOST:-${HARBOR_REGISTRY}}"
}

# (Re-)authenticate helm and cosign with Harbor.
harbor_reauth() {
  local host
  host="$(_harbor_host)"
  echo "::notice::Re-authenticating with Harbor (${host})..."
  echo "${HARBOR_REGISTRY_PASSWORD}" | \
    helm registry login "${host}" \
      -u "${HARBOR_REGISTRY_USER}" \
      --password-stdin || return $?
  # cosign login is optional -- only if cosign is available.
  if command -v cosign &>/dev/null; then
    echo "${HARBOR_REGISTRY_PASSWORD}" | \
      cosign login "${host}" \
        -u "${HARBOR_REGISTRY_USER}" \
        --password-stdin || return $?
  fi
}

# Retry a command with exponential backoff and re-auth before each retry.
# Usage: harbor_retry <description> <command ...>
harbor_retry() {
  local desc="$1"; shift
  local max_retries=${HARBOR_RETRY_MAX}
  local retry_delay=${HARBOR_RETRY_DELAY}

  for attempt in $(seq 1 "${max_retries}"); do
    echo "=== ${desc} (attempt ${attempt}/${max_retries}) ==="
    if "$@"; then
      return 0
    fi
    if [[ ${attempt} -lt ${max_retries} ]]; then
      echo "::warning::${desc} failed on attempt ${attempt}, re-authenticating and retrying in ${retry_delay}s..."
      harbor_reauth
      sleep "${retry_delay}"
      retry_delay=$((retry_delay * 2))
    fi
  done

  echo "::error::${desc} failed after ${max_retries} attempts"
  return 1
}

# Login to Harbor with retry (no re-auth needed, just retry the login itself).
harbor_login() {
  local max_retries=${HARBOR_RETRY_MAX}
  local retry_delay=${HARBOR_RETRY_DELAY}

  for attempt in $(seq 1 "${max_retries}"); do
    echo "=== Harbor login attempt ${attempt}/${max_retries} ==="
    if harbor_reauth; then
      echo "Harbor login succeeded on attempt ${attempt}"
      return 0
    fi
    if [[ ${attempt} -lt ${max_retries} ]]; then
      echo "::warning::Harbor login failed on attempt ${attempt}, retrying in ${retry_delay}s..."
      sleep "${retry_delay}"
      retry_delay=$((retry_delay * 2))
    fi
  done

  echo "::error::Harbor login failed after ${max_retries} attempts"
  return 1
}

# Common curl flags for Harbor API calls with retry.
# Usage: harbor_curl [extra-curl-args...]
#   e.g. harbor_curl -X POST "${url}" -H "Content-Type: application/json" -d '...'
harbor_curl() {
  curl -sf -u "${HARBOR_REGISTRY_USER}:${HARBOR_REGISTRY_PASSWORD}" \
    --retry 3 --retry-delay 5 --retry-all-errors \
    "$@"
}

# Helm pull with retry and re-auth on transient 401 errors.
# Returns 0 on success, 1 on non-auth failure (e.g. not found), 2 on persistent auth failure.
# Usage: harbor_helm_pull <helm-pull-args...>
harbor_helm_pull() {
  local max_retries=${HARBOR_RETRY_MAX}
  local retry_delay=${HARBOR_RETRY_DELAY}
  local pull_stderr
  pull_stderr=$(mktemp)
  # shellcheck disable=SC2064
  trap "rm -f '${pull_stderr}'" RETURN

  for attempt in $(seq 1 "${max_retries}"); do
    local exit_code=0
    helm pull "$@" 2>"${pull_stderr}" && exit_code=0 || exit_code=$?

    if [[ ${exit_code} -eq 0 ]]; then
      return 0
    fi

    # Auth error -> retry with re-auth; anything else -> return failure immediately
    if grep -qiE '401|unauthorized|unauthenticated' "${pull_stderr}"; then
      echo "::warning::helm pull auth error on attempt ${attempt}/${max_retries}"
      cat "${pull_stderr}" >&2
      if [[ ${attempt} -lt ${max_retries} ]]; then
        harbor_reauth
        sleep "${retry_delay}"
        retry_delay=$((retry_delay * 2))
        continue
      fi
      echo "::error::helm pull auth error persisted after ${max_retries} attempts"
      return 2
    fi

    # Non-auth error (e.g. not found)
    cat "${pull_stderr}" >&2
    return 1
  done
}
