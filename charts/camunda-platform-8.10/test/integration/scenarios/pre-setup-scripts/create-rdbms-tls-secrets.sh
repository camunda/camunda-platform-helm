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
# Generates a self-signed CA + server cert and creates the Kubernetes
# secrets required by the "rdbms-self-signed" persistence layer.
#
# Usage:
#   export TEST_NAMESPACE=<namespace>
#   export KUBE_CONTEXT=<context>           # optional
#   bash create-rdbms-tls-secrets.sh
#
# Creates two secrets:
#   rdbms-tls-server  - PEM cert + key + CA for the Bitnami postgresql
#                       companion (referenced via tls.certificatesSecret)
#   rdbms-tls-ca      - CA cert PEM only, mounted into Camunda
#                       components and referenced from the JDBC URL via
#                       sslrootcert=/etc/camunda/tls/ca.crt
#
# Prerequisites: openssl, kubectl
set -euo pipefail

NAMESPACE="${TEST_NAMESPACE:?TEST_NAMESPACE must be set}"
CONTEXT_FLAG=()
if [[ -n "${KUBE_CONTEXT:-}" ]]; then
  CONTEXT_FLAG=(--context "$KUBE_CONTEXT")
fi

CERT_VALIDITY_DAYS=365

WORK_DIR=$(mktemp -d)
trap '[[ -n "${WORK_DIR:-}" ]] && rm -rf "$WORK_DIR"' EXIT

echo "[rdbms-tls] Working directory: $WORK_DIR"

# ---------------------------------------------------------------------------
# 1. CA
# ---------------------------------------------------------------------------
echo "[rdbms-tls] Generating self-signed CA..."
openssl req -x509 -nodes -newkey rsa:4096 \
  -keyout "$WORK_DIR/ca.key" \
  -out "$WORK_DIR/ca.crt" \
  -days "$CERT_VALIDITY_DAYS" \
  -subj "/CN=postgres-tls-ca/O=camunda-ci" 2>/dev/null

# ---------------------------------------------------------------------------
# 2. Server cert with SANs matching the Bitnami postgresql service names.
#    Companion release name is "postgres-tls", so service is "postgres-tls"
#    and the headless service is "postgres-tls-hl".
# ---------------------------------------------------------------------------
echo "[rdbms-tls] Generating PostgreSQL server certificate..."
cat > "$WORK_DIR/server.cnf" <<EOF
[req]
distinguished_name = req_dn
req_extensions     = v3_req
prompt             = no

[req_dn]
CN = postgres-tls
O  = camunda-ci

[v3_req]
subjectAltName = @alt_names
extendedKeyUsage = serverAuth

[alt_names]
DNS.1 = postgres-tls
DNS.2 = postgres-tls.${NAMESPACE}.svc.cluster.local
DNS.3 = postgres-tls-hl
DNS.4 = postgres-tls-hl.${NAMESPACE}.svc.cluster.local
DNS.5 = postgres-tls-postgresql
DNS.6 = postgres-tls-postgresql.${NAMESPACE}.svc.cluster.local
DNS.7 = postgres-tls-postgresql-hl
DNS.8 = postgres-tls-postgresql-hl.${NAMESPACE}.svc.cluster.local
DNS.9 = localhost
IP.1  = 127.0.0.1
EOF

openssl req -nodes -newkey rsa:4096 \
  -keyout "$WORK_DIR/tls.key" \
  -out "$WORK_DIR/tls.csr" \
  -config "$WORK_DIR/server.cnf" 2>/dev/null

openssl x509 -req \
  -in "$WORK_DIR/tls.csr" \
  -CA "$WORK_DIR/ca.crt" \
  -CAkey "$WORK_DIR/ca.key" \
  -CAcreateserial \
  -out "$WORK_DIR/tls.crt" \
  -days "$CERT_VALIDITY_DAYS" \
  -extensions v3_req \
  -extfile "$WORK_DIR/server.cnf" 2>/dev/null

# ---------------------------------------------------------------------------
# 3. Create Kubernetes secrets (idempotent)
# ---------------------------------------------------------------------------
echo "[rdbms-tls] Creating Kubernetes secrets in namespace $NAMESPACE..."

create_or_replace_secret() {
  local name="$1"
  shift
  if kubectl ${CONTEXT_FLAG[@]+"${CONTEXT_FLAG[@]}"} -n "$NAMESPACE" get secret "$name" >/dev/null 2>&1; then
    echo "  Secret $name already exists — replacing"
    kubectl ${CONTEXT_FLAG[@]+"${CONTEXT_FLAG[@]}"} -n "$NAMESPACE" delete secret "$name" --ignore-not-found
  fi
  kubectl ${CONTEXT_FLAG[@]+"${CONTEXT_FLAG[@]}"} -n "$NAMESPACE" create secret generic "$name" "$@"
}

create_or_replace_secret "rdbms-tls-server" \
  --from-file="tls.crt=$WORK_DIR/tls.crt" \
  --from-file="tls.key=$WORK_DIR/tls.key" \
  --from-file="ca.crt=$WORK_DIR/ca.crt"

create_or_replace_secret "rdbms-tls-ca" \
  --from-file="ca.crt=$WORK_DIR/ca.crt"

echo "[rdbms-tls] Done. Created secrets:"
echo "  - rdbms-tls-server (PEM cert + key + CA for the postgresql companion)"
echo "  - rdbms-tls-ca     (CA cert PEM, mounted into Camunda components)"
