#!/bin/bash

set -euo pipefail

: "${TEST_NAMESPACE:?TEST_NAMESPACE must be set}"

for attempt in {1..60}; do
  if kubectl exec -n "${TEST_NAMESPACE}" deployment/postgresql -- \
    sh -c 'psql --username "$POSTGRES_USER" --dbname webmodeler --tuples-only --no-align --command "SELECT name FROM clusters WHERE name = '\''authenticated-hub-ping'\''"' \
    | grep -qx 'authenticated-hub-ping'; then
    echo "Authenticated Hub ping registered the orchestration cluster."
    exit 0
  fi

  echo "Waiting for authenticated Hub ping registration (attempt ${attempt}/60)..."
  sleep 5
done

echo "Authenticated Hub ping did not register the orchestration cluster." >&2
exit 1
