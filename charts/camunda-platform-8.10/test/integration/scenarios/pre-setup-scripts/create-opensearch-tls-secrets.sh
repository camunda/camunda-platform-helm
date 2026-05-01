#!/bin/bash
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
# Generates a self-signed CA + node + admin certificates and creates the
# Kubernetes secrets required by the "opensearch-self-signed" persistence layer.
#
# Usage:
#   export TEST_NAMESPACE=<namespace>
#   export KUBE_CONTEXT=<context>           # optional
#   bash create-opensearch-tls-secrets.sh
#
# Creates three secrets:
#   opensearch-tls-certs  - CA + node + admin PEM materials for the OS pods
#   opensearch-tls-ca     - CA cert (PEM) only, mounted into Camunda components
#                           and pointed at via SSL_CERT_FILE (the supported,
#                           non-root-init-container path documented in
#                           camunda/camunda-platform-helm#3498).
#   opensearch-jks        - JKS truststore + password, consumed by the chart's
#                           existing tls.secret.existingSecret wiring for
#                           Java HttpClient-based components.
#
# Prerequisites: openssl, keytool (JDK), kubectl
set -euo pipefail

NAMESPACE="${TEST_NAMESPACE:?TEST_NAMESPACE must be set}"
CONTEXT_FLAG=()
if [[ -n "${KUBE_CONTEXT:-}" ]]; then
  CONTEXT_FLAG=(--context "$KUBE_CONTEXT")
fi

CERT_VALIDITY_DAYS=365
TRUSTSTORE_PASSWORD="${TRUSTSTORE_PASSWORD:-changeit}"

WORK_DIR=$(mktemp -d)
trap '[[ -n "${WORK_DIR:-}" ]] && rm -rf "$WORK_DIR"' EXIT

echo "[opensearch-tls] Working directory: $WORK_DIR"

# ---------------------------------------------------------------------------
# 1. CA
# ---------------------------------------------------------------------------
echo "[opensearch-tls] Generating self-signed CA..."
openssl req -x509 -nodes -newkey rsa:4096 \
  -keyout "$WORK_DIR/ca.key" \
  -out "$WORK_DIR/ca.crt" \
  -days "$CERT_VALIDITY_DAYS" \
  -subj "/CN=opensearch-ca/O=camunda-ci" 2>/dev/null

# ---------------------------------------------------------------------------
# 2. Node (server) cert
# ---------------------------------------------------------------------------
echo "[opensearch-tls] Generating OpenSearch node certificate..."
cat > "$WORK_DIR/node.cnf" <<EOF
[req]
distinguished_name = req_dn
req_extensions     = v3_req
prompt             = no

[req_dn]
CN = opensearch-master
O  = camunda-ci

[v3_req]
subjectAltName = @alt_names
extendedKeyUsage = serverAuth, clientAuth

[alt_names]
# Service names are derived from the companion chart's masterService override
# (test/integration/companion-values/opensearch-tls.yaml sets
# masterService: opensearch-master, so the chart creates "opensearch-master"
# and "opensearch-master-headless"; the default "opensearch-cluster-master"
# service is NOT created and must not appear here).
DNS.1 = opensearch-master
DNS.2 = opensearch-master.${NAMESPACE}.svc.cluster.local
DNS.3 = opensearch-master-0.opensearch-master-headless.${NAMESPACE}.svc.cluster.local
DNS.4 = opensearch-master-headless
DNS.5 = opensearch-master-headless.${NAMESPACE}.svc.cluster.local
DNS.6 = localhost
IP.1  = 127.0.0.1
EOF

openssl req -nodes -newkey rsa:4096 \
  -keyout "$WORK_DIR/tls.key" \
  -out "$WORK_DIR/tls.csr" \
  -config "$WORK_DIR/node.cnf" 2>/dev/null

openssl x509 -req \
  -in "$WORK_DIR/tls.csr" \
  -CA "$WORK_DIR/ca.crt" \
  -CAkey "$WORK_DIR/ca.key" \
  -CAcreateserial \
  -out "$WORK_DIR/tls.crt" \
  -days "$CERT_VALIDITY_DAYS" \
  -extensions v3_req \
  -extfile "$WORK_DIR/node.cnf" 2>/dev/null

# ---------------------------------------------------------------------------
# 3. Admin cert (used by OS security plugin's securityadmin tooling and as the
#    privileged client identity when initialising the security index).
# ---------------------------------------------------------------------------
echo "[opensearch-tls] Generating OpenSearch admin certificate..."
cat > "$WORK_DIR/admin.cnf" <<EOF
[req]
distinguished_name = req_dn
prompt             = no

[req_dn]
CN = admin
O  = camunda-ci
EOF

openssl req -nodes -newkey rsa:4096 \
  -keyout "$WORK_DIR/admin.key" \
  -out "$WORK_DIR/admin.csr" \
  -config "$WORK_DIR/admin.cnf" 2>/dev/null

openssl x509 -req \
  -in "$WORK_DIR/admin.csr" \
  -CA "$WORK_DIR/ca.crt" \
  -CAkey "$WORK_DIR/ca.key" \
  -CAcreateserial \
  -out "$WORK_DIR/admin.crt" \
  -days "$CERT_VALIDITY_DAYS" 2>/dev/null

# ---------------------------------------------------------------------------
# 4. JKS truststore for Camunda components (legacy Java truststore path).
# ---------------------------------------------------------------------------
echo "[opensearch-tls] Creating JKS truststore (externaldb.jks) for Camunda..."
keytool -importcert \
  -keystore "$WORK_DIR/externaldb.jks" \
  -storetype JKS \
  -storepass "$TRUSTSTORE_PASSWORD" \
  -alias opensearch-ca \
  -file "$WORK_DIR/ca.crt" \
  -noprompt 2>/dev/null

# ---------------------------------------------------------------------------
# 5. Create Kubernetes secrets (idempotent)
# ---------------------------------------------------------------------------
echo "[opensearch-tls] Creating Kubernetes secrets in namespace $NAMESPACE..."

create_or_replace_secret() {
  local name="$1"
  shift
  if kubectl "${CONTEXT_FLAG[@]}" -n "$NAMESPACE" get secret "$name" >/dev/null 2>&1; then
    echo "  Secret $name already exists — replacing"
    kubectl "${CONTEXT_FLAG[@]}" -n "$NAMESPACE" delete secret "$name" --ignore-not-found
  fi
  kubectl "${CONTEXT_FLAG[@]}" -n "$NAMESPACE" create secret generic "$name" "$@"
}

create_or_replace_secret "opensearch-tls-certs" \
  --from-file="ca.crt=$WORK_DIR/ca.crt" \
  --from-file="tls.crt=$WORK_DIR/tls.crt" \
  --from-file="tls.key=$WORK_DIR/tls.key" \
  --from-file="admin.crt=$WORK_DIR/admin.crt" \
  --from-file="admin.key=$WORK_DIR/admin.key"

create_or_replace_secret "opensearch-tls-ca" \
  --from-file="ca.crt=$WORK_DIR/ca.crt"

create_or_replace_secret "opensearch-jks" \
  --from-file="externaldb.jks=$WORK_DIR/externaldb.jks" \
  --from-literal="truststore-password=$TRUSTSTORE_PASSWORD"

echo "[opensearch-tls] Done. Created secrets:"
echo "  - opensearch-tls-certs (CA + node + admin PEM material for OS pods)"
echo "  - opensearch-tls-ca    (CA cert PEM, for SSL_CERT_FILE on Camunda)"
echo "  - opensearch-jks       (JKS truststore for legacy Java client path)"
