#!/bin/bash
# Copyright 2025 Camunda Services GmbH
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
# Increases Keycloak realm token lifespans for CI/E2E testing stability.
#
# The default access token lifespan (5 min) can expire during long-running
# E2E test flows (diagram building, process deployment) when the cluster is
# under heavy load. This script patches the camunda-platform realm to use
# 30-minute access tokens and 2-hour SSO sessions, eliminating session
# expiry as a source of test flakiness.
#
# Usage (called automatically by deploy-camunda as a post-deploy hook):
#   export TEST_NAMESPACE=<namespace>
#   export KUBE_CONTEXT=<context>           # optional
#   export RELEASE_NAME=<release>           # optional, defaults to "integration"
#   bash post-deploy-keycloak-token-lifespan.sh
#
# Prerequisites: kubectl, curl, jq
#
set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------
NAMESPACE="${TEST_NAMESPACE:?TEST_NAMESPACE must be set}"
RELEASE="${RELEASE_NAME:-integration}"
CONTEXT_FLAG=""
if [[ -n "${KUBE_CONTEXT:-}" ]]; then
  CONTEXT_FLAG="--context ${KUBE_CONTEXT}"
fi

REALM="camunda-platform"
# 30-minute access tokens (default: 5 min) — long enough for any E2E test flow.
ACCESS_TOKEN_LIFESPAN=1800
# 2-hour SSO session idle timeout (default: 30 min).
SSO_SESSION_IDLE_TIMEOUT=7200
# 12-hour SSO session max (default: 10 hours).
SSO_SESSION_MAX_LIFESPAN=43200

echo "[keycloak-token-lifespan] Patching realm '${REALM}' in namespace ${NAMESPACE}..."

# ---------------------------------------------------------------------------
# 1. Get Keycloak admin password from K8s secret
# ---------------------------------------------------------------------------
KEYCLOAK_PASSWORD=$(kubectl ${CONTEXT_FLAG} -n "${NAMESPACE}" get secret integration-test-credentials \
  -o jsonpath='{.data.identity-keycloak-admin-password}' | base64 -d)

if [[ -z "${KEYCLOAK_PASSWORD}" ]]; then
  echo "[keycloak-token-lifespan] ERROR: Could not retrieve Keycloak admin password from secret"
  exit 1
fi

# ---------------------------------------------------------------------------
# 2. Port-forward to Keycloak and obtain admin token
# ---------------------------------------------------------------------------
LOCAL_PORT=18080
kubectl ${CONTEXT_FLAG} -n "${NAMESPACE}" port-forward "svc/${RELEASE}-keycloak" "${LOCAL_PORT}:80" &
PF_PID=$!
trap 'kill $PF_PID 2>/dev/null || true' EXIT

# Wait for port-forward to be ready
for i in $(seq 1 30); do
  if curl -sf "http://localhost:${LOCAL_PORT}/auth/" >/dev/null 2>&1; then
    break
  fi
  if [[ $i -eq 30 ]]; then
    echo "[keycloak-token-lifespan] ERROR: Port-forward to Keycloak not ready after 30s"
    exit 1
  fi
  sleep 1
done

KEYCLOAK_URL="http://localhost:${LOCAL_PORT}/auth"

# Get admin access token
TOKEN_RESPONSE=$(curl -sf -X POST "${KEYCLOAK_URL}/realms/master/protocol/openid-connect/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "username=admin" \
  -d "password=${KEYCLOAK_PASSWORD}" \
  -d "grant_type=password" \
  -d "client_id=admin-cli")

ADMIN_TOKEN=$(echo "${TOKEN_RESPONSE}" | jq -r '.access_token')

if [[ -z "${ADMIN_TOKEN}" || "${ADMIN_TOKEN}" == "null" ]]; then
  echo "[keycloak-token-lifespan] ERROR: Failed to obtain admin token"
  echo "${TOKEN_RESPONSE}"
  exit 1
fi

# ---------------------------------------------------------------------------
# 3. Patch realm token/session lifespans
# ---------------------------------------------------------------------------
HTTP_CODE=$(curl -sf -o /dev/null -w "%{http_code}" -X PUT \
  "${KEYCLOAK_URL}/admin/realms/${REALM}" \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H "Content-Type: application/json" \
  -d "{
    \"accessTokenLifespan\": ${ACCESS_TOKEN_LIFESPAN},
    \"ssoSessionIdleTimeout\": ${SSO_SESSION_IDLE_TIMEOUT},
    \"ssoSessionMaxLifespan\": ${SSO_SESSION_MAX_LIFESPAN}
  }")

if [[ "${HTTP_CODE}" == "204" ]]; then
  echo "[keycloak-token-lifespan] Successfully patched realm '${REALM}':"
  echo "  accessTokenLifespan:    ${ACCESS_TOKEN_LIFESPAN}s (30 min)"
  echo "  ssoSessionIdleTimeout:  ${SSO_SESSION_IDLE_TIMEOUT}s (2 hours)"
  echo "  ssoSessionMaxLifespan:  ${SSO_SESSION_MAX_LIFESPAN}s (12 hours)"
else
  echo "[keycloak-token-lifespan] WARNING: Realm patch returned HTTP ${HTTP_CODE}"
  echo "  This is non-fatal — E2E tests have re-login handling as a fallback."
fi
