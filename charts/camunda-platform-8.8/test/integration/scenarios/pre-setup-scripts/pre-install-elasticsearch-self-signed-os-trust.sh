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
# Pre-install for the "elasticsearch-self-signed-os-trust" persistence layer.
#
# openssl-ONLY (deliberately no keytool/JDK): the matrix pre-install runner does
# not ship a JDK, so we cannot build JKS keystores here. Instead we generate a
# self-signed CA and an ES node cert/key as PEM, and run the bundled
# Elasticsearch in PEM mode (security.tls.usePemCerts=true). The SAME ca.crt is
# also consumed by global.tls.caBundle, so the Camunda components trust the ES
# endpoint solely via the chart-built JVM truststore — no per-component JKS.
#
# Required env vars (set by the matrix runner):
#   TEST_NAMESPACE  - target Kubernetes namespace
#   KUBE_CONTEXT    - kubectl context (optional)
#   RELEASE_NAME    - helm release name (optional, defaults to "integration")
#
# Creates one secret:
#   elasticsearch-tls-pem  - keys: tls.crt, tls.key (ES node PEM certs),
#                            ca.crt (ES trust + global.tls.caBundle input)
#
# Prerequisites: openssl, kubectl (NO keytool/JDK needed).

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
echo "[es-pem] Working directory: $WORK_DIR"

# ---------------------------------------------------------------------------
# 1. Self-signed CA
# ---------------------------------------------------------------------------
echo "[es-pem] Generating self-signed CA..."
openssl req -x509 -nodes -newkey rsa:4096 \
  -keyout "$WORK_DIR/ca.key" \
  -out "$WORK_DIR/ca.crt" \
  -days "$CERT_VALIDITY_DAYS" \
  -subj "/CN=elasticsearch-ca/O=camunda-ci" \
  2>/dev/null

# ---------------------------------------------------------------------------
# 2. ES node certificate (PEM) — SANs cover the master pods + the service name
#    the Camunda components connect to (<release>-elasticsearch).
# ---------------------------------------------------------------------------
cat > "$WORK_DIR/san.cnf" <<EOF
[req]
distinguished_name = req_dn
req_extensions     = v3_req
prompt             = no

[req_dn]
CN = ${RELEASE}-elasticsearch-master

[v3_req]
subjectAltName = @alt_names

[alt_names]
DNS.1 = ${RELEASE}-elasticsearch-master
DNS.2 = ${RELEASE}-elasticsearch-master-0.${RELEASE}-elasticsearch-master-hl.${NAMESPACE}.svc.cluster.local
DNS.3 = ${RELEASE}-elasticsearch-master-1.${RELEASE}-elasticsearch-master-hl.${NAMESPACE}.svc.cluster.local
DNS.4 = ${RELEASE}-elasticsearch-master-2.${RELEASE}-elasticsearch-master-hl.${NAMESPACE}.svc.cluster.local
DNS.5 = ${RELEASE}-elasticsearch-master-hl.${NAMESPACE}.svc.cluster.local
DNS.6 = ${RELEASE}-elasticsearch
DNS.7 = ${RELEASE}-elasticsearch.${NAMESPACE}.svc.cluster.local
DNS.8 = localhost
IP.1  = 127.0.0.1
EOF

echo "[es-pem] Generating ES node certificate (PEM)..."
openssl req -nodes -newkey rsa:4096 \
  -keyout "$WORK_DIR/tls.key" \
  -out "$WORK_DIR/node.csr" \
  -config "$WORK_DIR/san.cnf" \
  2>/dev/null

openssl x509 -req \
  -in "$WORK_DIR/node.csr" \
  -CA "$WORK_DIR/ca.crt" \
  -CAkey "$WORK_DIR/ca.key" \
  -CAcreateserial \
  -out "$WORK_DIR/tls.crt" \
  -days "$CERT_VALIDITY_DAYS" \
  -extensions v3_req \
  -extfile "$WORK_DIR/san.cnf" \
  2>/dev/null

# ---------------------------------------------------------------------------
# 3. Create the PEM secret (idempotent)
# ---------------------------------------------------------------------------
echo "[es-pem] Creating secret elasticsearch-tls-pem in namespace $NAMESPACE..."
kubectl ${CONTEXT_FLAG} -n "$NAMESPACE" delete secret elasticsearch-tls-pem --ignore-not-found >/dev/null 2>&1 || true
kubectl ${CONTEXT_FLAG} -n "$NAMESPACE" create secret generic elasticsearch-tls-pem \
  --from-file="tls.crt=$WORK_DIR/tls.crt" \
  --from-file="tls.key=$WORK_DIR/tls.key" \
  --from-file="ca.crt=$WORK_DIR/ca.crt"

echo "[es-pem] Done. Created secret elasticsearch-tls-pem (tls.crt, tls.key, ca.crt) — no keytool/JKS."
