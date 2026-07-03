#!/bin/bash
# Copyright 2024 Camunda Services GmbH
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
# Generates a self-signed certificate and creates the Kubernetes secret used by
# Zeebe internal TLS migration tests.

set -euo pipefail

NAMESPACE="${TEST_NAMESPACE:?TEST_NAMESPACE must be set}"
RELEASE="${RELEASE_NAME:-integration}"
CONTEXT_FLAG=""
if [[ -n "${KUBE_CONTEXT:-}" ]]; then
  CONTEXT_FLAG="--context ${KUBE_CONTEXT}"
fi

CERT_VALIDITY_DAYS=365

WORK_DIR=$(mktemp -d)
trap '[[ -n "${WORK_DIR:-}" ]] && rm -rf "$WORK_DIR"' EXIT

echo "[zeebe-tls] Working directory: $WORK_DIR"
echo "[zeebe-tls] Generating self-signed certificate..."

cat > "$WORK_DIR/san.cnf" <<EOF
[req]
distinguished_name = req_dn
req_extensions     = v3_req
prompt             = no

[req_dn]
CN = ${RELEASE}-zeebe
O  = camunda-ci

[v3_req]
subjectAltName = @alt_names

[alt_names]
DNS.1  = ${RELEASE}-zeebe
DNS.2  = ${RELEASE}-zeebe.${NAMESPACE}.svc.cluster.local
DNS.3  = ${RELEASE}-zeebe-gateway
DNS.4  = ${RELEASE}-zeebe-gateway.${NAMESPACE}.svc.cluster.local
DNS.5  = ${RELEASE}-zeebe-0.${RELEASE}-zeebe
DNS.6  = ${RELEASE}-zeebe-1.${RELEASE}-zeebe
DNS.7  = ${RELEASE}-zeebe-2.${RELEASE}-zeebe
DNS.8  = ${RELEASE}-zeebe-0.${RELEASE}-zeebe.${NAMESPACE}.svc.cluster.local
DNS.9  = ${RELEASE}-zeebe-1.${RELEASE}-zeebe.${NAMESPACE}.svc.cluster.local
DNS.10 = ${RELEASE}-zeebe-2.${RELEASE}-zeebe.${NAMESPACE}.svc.cluster.local
DNS.11 = localhost
IP.1   = 127.0.0.1
EOF

openssl req -nodes -newkey rsa:4096 \
  -keyout "$WORK_DIR/zeebeCluster.key" \
  -out "$WORK_DIR/zeebeCluster.csr" \
  -config "$WORK_DIR/san.cnf" \
  2>/dev/null

openssl x509 -req \
  -in "$WORK_DIR/zeebeCluster.csr" \
  -signkey "$WORK_DIR/zeebeCluster.key" \
  -out "$WORK_DIR/chainZeebeCluster.pem" \
  -days "$CERT_VALIDITY_DAYS" \
  -extensions v3_req \
  -extfile "$WORK_DIR/san.cnf" \
  2>/dev/null

echo "[zeebe-tls] Creating Kubernetes secret in namespace $NAMESPACE..."

if kubectl ${CONTEXT_FLAG} -n "$NAMESPACE" get secret camunda-zeebe-tls >/dev/null 2>&1; then
  echo "  Secret camunda-zeebe-tls already exists - replacing"
  kubectl ${CONTEXT_FLAG} -n "$NAMESPACE" delete secret camunda-zeebe-tls --ignore-not-found
fi

kubectl ${CONTEXT_FLAG} -n "$NAMESPACE" create secret generic camunda-zeebe-tls \
  --from-file="chainZeebeCluster.pem=$WORK_DIR/chainZeebeCluster.pem" \
  --from-file="zeebeCluster.key=$WORK_DIR/zeebeCluster.key"

echo "[zeebe-tls] Done. Created secret camunda-zeebe-tls."
