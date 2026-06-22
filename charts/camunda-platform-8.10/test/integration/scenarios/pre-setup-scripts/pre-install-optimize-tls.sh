#!/bin/bash
# Copyright Camunda Services GmbH and/or licensed to Camunda Services GmbH
# under one or more contributor license agreements. Licensed under a proprietary license.
#
# TEST FIXTURE — DO NOT USE IN PRODUCTION
# Generates a self-signed RSA-4096 cert for the Optimize HTTP server,
# packages it as a PKCS12 keystore with the literal password "changeit",
# and creates a `optimize-tls-keystore` Secret in the scenario namespace.

set -euo pipefail

NAMESPACE="${TEST_NAMESPACE:?TEST_NAMESPACE must be set}"
RELEASE="${RELEASE_NAME:-integration}"
CONTEXT_FLAG=""
if [[ -n "${KUBE_CONTEXT:-}" ]]; then
  CONTEXT_FLAG="--context ${KUBE_CONTEXT}"
fi

CERT_VALIDITY_DAYS=365
KEYSTORE_PASSWORD="changeit"
WORK_DIR=$(mktemp -d)
trap '[[ -n "${WORK_DIR:-}" ]] && rm -rf "$WORK_DIR"' EXIT

cat > "$WORK_DIR/san.cnf" <<EOF
[req]
distinguished_name = req_dn
req_extensions = v3_req
prompt = no

[req_dn]
CN = ${RELEASE}-optimize

[v3_req]
subjectAltName = @alt_names

[alt_names]
DNS.1 = ${RELEASE}-optimize
DNS.2 = ${RELEASE}-optimize.${NAMESPACE}.svc
DNS.3 = ${RELEASE}-optimize.${NAMESPACE}.svc.cluster.local
DNS.4 = ${RELEASE}-optimize-headless
DNS.5 = ${RELEASE}-optimize-headless.${NAMESPACE}.svc
DNS.6 = localhost
IP.1 = 127.0.0.1
EOF

openssl req -x509 -nodes -newkey rsa:4096 \
  -keyout "$WORK_DIR/tls.key" \
  -out "$WORK_DIR/tls.crt" \
  -days "$CERT_VALIDITY_DAYS" \
  -config "$WORK_DIR/san.cnf" \
  -extensions v3_req \
  2>/dev/null

openssl pkcs12 -export \
  -in "$WORK_DIR/tls.crt" \
  -inkey "$WORK_DIR/tls.key" \
  -out "$WORK_DIR/keystore.p12" \
  -password "pass:$KEYSTORE_PASSWORD" \
  -name optimize-rest \
  2>/dev/null

create_or_replace_secret() {
  local name="$1"
  shift
  if kubectl ${CONTEXT_FLAG} -n "$NAMESPACE" get secret "$name" >/dev/null 2>&1; then
    kubectl ${CONTEXT_FLAG} -n "$NAMESPACE" delete secret "$name" --ignore-not-found
  fi
  kubectl ${CONTEXT_FLAG} -n "$NAMESPACE" create secret generic "$name" "$@"
}

create_or_replace_secret "optimize-tls-keystore" \
  --from-file="keystore.p12=$WORK_DIR/keystore.p12" \
  --from-literal="keystore-password=$KEYSTORE_PASSWORD"
