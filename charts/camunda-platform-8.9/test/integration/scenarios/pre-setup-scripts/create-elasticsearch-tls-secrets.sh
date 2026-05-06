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
# Generates self-signed TLS certificates and creates the Kubernetes secrets
# required by the "elasticsearch-self-signed" persistence layer.
#
# Usage:
#   export TEST_NAMESPACE=<namespace>
#   export KUBE_CONTEXT=<context>           # optional
#   export RELEASE_NAME=<release>           # optional, defaults to "integration"
#   bash create-elasticsearch-tls-secrets.sh
#
# Creates three secrets:
#   elasticsearch-tls-certs     - JKS keystore + truststore for ES nodes
#   elasticsearch-tls-passwords - keystore/truststore passwords for ES nodes
#   elasticsearch-jks           - JKS truststore for Camunda components
#
# Prerequisites: openssl, keytool (JDK), kubectl
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

KEYSTORE_PASSWORD="changeit"
TRUSTSTORE_PASSWORD="changeit"
CERT_VALIDITY_DAYS=365

# Temp dir for generated artifacts — cleaned up on exit.
WORK_DIR=$(mktemp -d)
trap '[[ -n "${WORK_DIR:-}" ]] && rm -rf "$WORK_DIR"' EXIT

echo "[elasticsearch-tls] Working directory: $WORK_DIR"

# ---------------------------------------------------------------------------
# 1. Generate CA
# ---------------------------------------------------------------------------
echo "[elasticsearch-tls] Generating self-signed CA..."
openssl req -x509 -nodes -newkey rsa:4096 \
  -keyout "$WORK_DIR/ca.key" \
  -out "$WORK_DIR/ca.crt" \
  -days "$CERT_VALIDITY_DAYS" \
  -subj "/CN=elasticsearch-ca/O=camunda-ci" \
  2>/dev/null

# ---------------------------------------------------------------------------
# 2. Generate node certificate
# ---------------------------------------------------------------------------
echo "[elasticsearch-tls] Generating ES node certificate..."

# SAN entries for Elasticsearch pod DNS names.
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

openssl req -nodes -newkey rsa:4096 \
  -keyout "$WORK_DIR/node.key" \
  -out "$WORK_DIR/node.csr" \
  -config "$WORK_DIR/san.cnf" \
  2>/dev/null

openssl x509 -req \
  -in "$WORK_DIR/node.csr" \
  -CA "$WORK_DIR/ca.crt" \
  -CAkey "$WORK_DIR/ca.key" \
  -CAcreateserial \
  -out "$WORK_DIR/node.crt" \
  -days "$CERT_VALIDITY_DAYS" \
  -extensions v3_req \
  -extfile "$WORK_DIR/san.cnf" \
  2>/dev/null

# ---------------------------------------------------------------------------
# 3. Create JKS stores
# ---------------------------------------------------------------------------
echo "[elasticsearch-tls] Creating JKS keystore..."

# PKCS12 bundle for keytool import.
openssl pkcs12 -export \
  -in "$WORK_DIR/node.crt" \
  -inkey "$WORK_DIR/node.key" \
  -certfile "$WORK_DIR/ca.crt" \
  -out "$WORK_DIR/node.p12" \
  -password "pass:${KEYSTORE_PASSWORD}" \
  -name es-node

keytool -importkeystore \
  -srckeystore "$WORK_DIR/node.p12" \
  -srcstoretype PKCS12 \
  -srcstorepass "$KEYSTORE_PASSWORD" \
  -destkeystore "$WORK_DIR/elasticsearch.keystore.jks" \
  -deststoretype JKS \
  -deststorepass "$KEYSTORE_PASSWORD" \
  -noprompt 2>/dev/null

echo "[elasticsearch-tls] Creating JKS truststores..."

# Truststore for ES nodes (contains CA cert).
# storepass uses $TRUSTSTORE_PASSWORD so the actual truststore password matches the
# value published into the "truststore-password" secret key below; if these diverge,
# Camunda components will fail to load the truststore at runtime.
keytool -importcert \
  -keystore "$WORK_DIR/elasticsearch.truststore.jks" \
  -storetype JKS \
  -storepass "$TRUSTSTORE_PASSWORD" \
  -alias ca \
  -file "$WORK_DIR/ca.crt" \
  -noprompt 2>/dev/null

# Truststore for Camunda components (same CA, key name externaldb.jks).
cp "$WORK_DIR/elasticsearch.truststore.jks" "$WORK_DIR/externaldb.jks"

# ---------------------------------------------------------------------------
# 4. Create Kubernetes secrets (idempotent)
# ---------------------------------------------------------------------------
echo "[elasticsearch-tls] Creating Kubernetes secrets in namespace $NAMESPACE..."

create_or_replace_secret() {
  local name="$1"
  shift
  if kubectl ${CONTEXT_FLAG} -n "$NAMESPACE" get secret "$name" >/dev/null 2>&1; then
    echo "  Secret $name already exists — replacing"
    kubectl ${CONTEXT_FLAG} -n "$NAMESPACE" delete secret "$name" --ignore-not-found
  fi
  kubectl ${CONTEXT_FLAG} -n "$NAMESPACE" create secret generic "$name" "$@"
}

create_or_replace_secret "elasticsearch-tls-certs" \
  --from-file="elasticsearch.keystore.jks=$WORK_DIR/elasticsearch.keystore.jks" \
  --from-file="elasticsearch.truststore.jks=$WORK_DIR/elasticsearch.truststore.jks"

create_or_replace_secret "elasticsearch-tls-passwords" \
  --from-literal="keystore-password=$KEYSTORE_PASSWORD" \
  --from-literal="truststore-password=$TRUSTSTORE_PASSWORD"

create_or_replace_secret "elasticsearch-jks" \
  --from-file="externaldb.jks=$WORK_DIR/externaldb.jks" \
  --from-literal="truststore-password=$TRUSTSTORE_PASSWORD"

echo "[elasticsearch-tls] Done. Created secrets:"
echo "  - elasticsearch-tls-certs     (JKS keystore + truststore for ES nodes)"
echo "  - elasticsearch-tls-passwords (passwords for ES node stores)"
echo "  - elasticsearch-jks           (JKS truststore for Camunda components)"
