#!/usr/bin/env bash
# post-infra hook — Bitnami → companion data migration for the 8.9→8.10 upgrade.
#
# Runs AFTER the upgrade scenario's companion charts (internal-postgresql →
# Service "postgresql", internal-keycloak-26 → Service "keycloak") are deployed
# and ready, but BEFORE the matrix runner upgrades the chart to 8.10 (which
# removes the bundled Bitnami subcharts). It uses the official
# camunda-deployment-references migration scripts in EXTERNAL target mode with
# SKIP_HELM_UPGRADE=true, so the realm / Identity / Web Modeler databases are
# moved off the bundled Bitnami backends onto the companion services while the
# chart upgrade itself stays the runner's job.
#
# Env provided by the matrix lifecycle runner (see lifecycle_hook.go):
#   NAMESPACE / TEST_NAMESPACE, RELEASE_NAME, KUBE_CONTEXT,
#   RDBMS_POSTGRESQL_USERNAME, RDBMS_POSTGRESQL_PASSWORD
#
# Pin the migration tooling. Defaults to stable/8.9 — the branch that
# camunda-deployment-references#2620 (KEYCLOAK_TARGET_MODE=external +
# SKIP_HELM_UPGRADE) merges into, and where the migration tooling now lives
# (it was removed from main). Pin to a tag/SHA once a release is cut.
set -euo pipefail

REF="${CAMUNDA_DEPLOYMENT_REFERENCES_REF:-stable/8.9}"
REPO="${CAMUNDA_DEPLOYMENT_REFERENCES_REPO:-https://github.com/camunda/camunda-deployment-references.git}"
NS="${NAMESPACE:-${TEST_NAMESPACE}}"
RELEASE="${RELEASE_NAME:-integration}"

workdir="$(mktemp -d)"
trap 'rm -rf "${workdir}"' EXIT

echo "Cloning ${REPO}@${REF} ..."
git clone --depth 1 --branch "${REF}" "${REPO}" "${workdir}/refs"
migration_dir="${workdir}/refs/generic/kubernetes/migration"

# --- Secrets the external-mode migration validates -------------------------
# Identity and Web Modeler databases live on the shared "postgresql" companion
# (internal-postgresql, RDBMS_POSTGRESQL_* credentials). Keycloak's realm DB
# lives on the companion Keycloak's OWN bundled PostgreSQL ("keycloak-postgresql",
# db/user "keycloak"), so the realm restore lands where the companion Keycloak
# actually reads from. The migration looks up one secret per component.
for secret in external-pg-identity external-pg-webmodeler; do
    kubectl create secret generic "${secret}" -n "${NS}" \
        --from-literal=password="${RDBMS_POSTGRESQL_PASSWORD}" \
        --dry-run=client -o yaml | kubectl apply -f -
done
# Keycloak DB password = the companion Keycloak's bundled-PG password
# (keycloak-qa.yaml: postgresql.auth.password). Read it from the companion's
# own secret so the value is never hardcoded.
kc_pg_pass="$(kubectl get secret keycloak-postgresql -n "${NS}" \
    -o jsonpath='{.data.postgresql-password}' 2>/dev/null | base64 -d || true)"
kubectl create secret generic external-pg-keycloak -n "${NS}" \
    --from-literal=password="${kc_pg_pass}" \
    --dry-run=client -o yaml | kubectl apply -f -

# --- Migration configuration (external mode, data-only) --------------------
export NAMESPACE="${NS}"
export CAMUNDA_RELEASE_NAME="${RELEASE}"
export AUTO_CONFIRM=true
export SKIP_HELM_UPGRADE=true

export PG_TARGET_MODE=external
export KEYCLOAK_TARGET_MODE=external

# The bundled Camunda 8.9 chart ships ONLY a Keycloak PostgreSQL (Bitnami
# defaults: db "bitnami_keycloak", user "bn_keycloak"). Identity has no
# dedicated PG (its data lives in Elasticsearch), and Web Modeler reinitialises
# its schema on first boot — so only the Keycloak realm needs PG migration.
export MIGRATE_KEYCLOAK=true
export MIGRATE_IDENTITY=false
export MIGRATE_WEBMODELER=false
# Elasticsearch holds the orchestration data AND the 8.10 authorization records
# (camunda-authorization/user/role/mapping-rule indices). They must move from
# the bundled Bitnami ES to the companion ES, or post-upgrade users get
# "you don't have access to this component" in Operate/Tasklist. Use warm
# reindex (the only automated external-ES path) from the bundled ES.
export MIGRATE_ELASTICSEARCH=true
export ES_TARGET_MODE=external
export ES_WARM_REINDEX=true
export EXTERNAL_ES_HOST=elasticsearch-master
export EXTERNAL_ES_PORT=9200
# Index patterns to reindex. The bundled Camunda indices are unprefixed in this
# scenario (camunda-authorization-*, camunda-user-*, operate-*, tasklist-*,
# optimize-*, zeebe-*). camunda-* carries the authorization/user/role/mapping
# records that gate Operate/Tasklist access after upgrade. The companion ES
# allows reindex-from-remote via reindex.remote.whitelist (companion-values).
export ES_INDEX_PREFIXES="camunda- operate- optimize- tasklist- zeebe-"

# Keycloak realm DB: migrate the bundled source (bitnami_keycloak / bn_keycloak)
# into the companion Keycloak's own bundled PostgreSQL (keycloak-postgresql,
# db/user "keycloak") — which is exactly what the companion Keycloak reads from.
export EXTERNAL_PG_KEYCLOAK_HOST=keycloak-postgresql
export EXTERNAL_PG_KEYCLOAK_SECRET=external-pg-keycloak
export KEYCLOAK_SOURCE_DB_NAME=bitnami_keycloak
export KEYCLOAK_SOURCE_DB_USER=bn_keycloak
export KEYCLOAK_DB_NAME=keycloak
export KEYCLOAK_DB_USER=keycloak

# Companion Keycloak ("keycloak" Service, /auth context path, internal HTTP:80).
# The realm is restored into the companion Keycloak's own bundled PG
# (keycloak-postgresql, configured above), which is exactly what this instance
# reads from — so the migrated realm is served without any companion-values change.
# Admin credentials are owned by the post-migration chart upgrade (the runner
# wires global.identity.keycloak.auth from integration-test-credentials); the
# data-only migration scripts do not consume a Keycloak admin secret.
export EXTERNAL_KEYCLOAK_PROTOCOL=http
export EXTERNAL_KEYCLOAK_HOST=keycloak
export EXTERNAL_KEYCLOAK_PORT=80
export EXTERNAL_KEYCLOAK_CONTEXT_PATH=/auth

cd "${migration_dir}"
# The phase scripts expect env.sh to have been sourced (it fills defaults such as
# CAMUNDA_HELM_CHART_VERSION). Our overrides above are preserved because env.sh
# assigns every variable with a ${VAR:-default} guard.
# shellcheck disable=SC1091
source ./env.sh
bash 1-deploy-targets.sh --yes
bash 2-backup.sh --yes

# Stop JDBC_PING writes while the realm database is restored.
echo "Stopping companion Keycloak for realm migration ..."
kubectl scale deployment/keycloak -n "${NS}" --replicas=0
kubectl rollout status deployment/keycloak -n "${NS}" --timeout=300s

bash 3-cutover.sh --yes

# Start one fresh pod to load the restored realm before the chart upgrade.
echo "Starting companion Keycloak with the migrated realm ..."
kubectl scale deployment/keycloak -n "${NS}" --replicas=1
kubectl rollout status deployment/keycloak -n "${NS}" --timeout=300s

echo "post-infra migration complete — realm moved to companion Keycloak; chart upgrade to 8.10 is the runner's job."
