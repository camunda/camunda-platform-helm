# Pre-built Keycloak image for CI integration tests
# Runs `kc.sh build` at image build time so the container can start with
# `--optimized`, skipping the ~30-40s Quarkus augmentation on every boot.
#
# The build-time options here MUST match what the Helm chart sets at runtime.
# If they drift, Keycloak will detect "changes" and re-augment anyway.
#
# Runtime config reference (from camunda-platform chart + test values overlay):
#   cache.enabled: false          -> KC_CACHE_TYPE=local  -> KC_CACHE=local (env var, not CLI flag)
#   httpRelativePath: /auth/      -> KC_HTTP_RELATIVE_PATH -> --http-relative-path=/auth/
#   metrics.enabled: false        -> KC_METRICS_ENABLED    -> --metrics-enabled=false
#   KC_HEALTH_ENABLED: true       -> extraEnvVars          -> --health-enabled=true
#   KC_TRANSACTION_XA_ENABLED: false -> extraEnvVars       -> --transaction-xa-enabled=false
#   KC_FEATURES_DISABLED: ...     -> extraEnvVars          -> --features-disabled=...
#   production: true              -> KEYCLOAK_PRODUCTION   -> kc.sh start (not start-dev)
#   tls.enabled: false            -> KC_HTTP_ENABLED=true  -> runtime env var (not a build flag)

ARG KEYCLOAK_IMAGE=camunda/keycloak
ARG KEYCLOAK_VERSION=26.3.3

FROM ${KEYCLOAK_IMAGE}:${KEYCLOAK_VERSION}

# Switch to root to run the build (kc.sh build needs write access to lib/quarkus)
USER 0

# Set build-time options that are only configurable via environment variables.
# --cache is a runtime option, not a build CLI flag, so it must be set as
# KC_CACHE before running kc.sh build.
ENV KC_CACHE=local

# Run Quarkus augmentation with the exact build-time options used in CI.
# This bakes the optimized Quarkus application into the image so `--optimized`
# can skip it at runtime.
#
# Valid kc.sh build flags (from `kc.sh build --help-all`):
#   --db, --transaction-xa-enabled, --features-disabled, --http-relative-path,
#   --health-enabled, --metrics-enabled
# NOT valid as build flags (must be env vars or runtime args):
#   --cache (use KC_CACHE env), --http-enabled (runtime only)
RUN /opt/bitnami/keycloak/bin/kc.sh build \
    --db=postgres \
    --http-relative-path=/auth/ \
    --health-enabled=true \
    --metrics-enabled=false \
    --features-disabled=ciba,client-policies,dpop,dynamic-scopes,kerberos,par,step-up-authentication,web-authn \
    --transaction-xa-enabled=false

# Switch back to the default non-root user (Bitnami default: 1001)
USER 1001
