#!/usr/bin/env bats

setup() {
  ROOT="$(git -C "$(dirname "${BATS_TEST_FILENAME}")" rev-parse --show-toplevel)"
  SCRIPT="$ROOT/charts/camunda-platform-8.10/test/integration/scenarios/pre-setup-scripts/post-deploy-physical-tenant-exporters.sh"
  eval "$(sed -n '/^partitions_are_healthy() {/,/^}/p' "$SCRIPT")"
  eval "$(sed -n '/^jwt_payload() {/,/^}/p' "$SCRIPT")"
  eval "$(sed -n '/^token_has_expected_audience() {/,/^}/p' "$SCRIPT")"
  eval "$(sed -n '/^definitions_contain() {/,/^}/p' "$SCRIPT")"
  eval "$(sed -n '/^tenant_api_path() {/,/^}/p' "$SCRIPT")"
}

encode_jwt_payload() {
  printf '%s' "$1" | base64 | tr -d '=\n' | tr '/+' '_-'
}

@test "partition validation accepts the complete healthy tenant set" {
  run partitions_are_healthy <<'EOF'
{"default":[{"exporterPhase":"EXPORTING","exportedPosition":1}],"tenanta":[{"exporterPhase":"EXPORTING","exportedPosition":2}],"tenantb":[{"exporterPhase":"EXPORTING","exportedPosition":3}]}
EOF

  [ "$status" -eq 0 ]
}

@test "partition validation rejects omission of either configured tenant" {
  run partitions_are_healthy <<'EOF'
{"default":[{"exporterPhase":"EXPORTING","exportedPosition":1}],"tenanta":[{"exporterPhase":"EXPORTING","exportedPosition":2}]}
EOF
  [ "$status" -ne 0 ]

  run partitions_are_healthy <<'EOF'
{"default":[{"exporterPhase":"EXPORTING","exportedPosition":1}],"tenantb":[{"exporterPhase":"EXPORTING","exportedPosition":3}]}
EOF
  [ "$status" -ne 0 ]
}

@test "partition validation rejects a tenant without partitions" {
  run partitions_are_healthy <<'EOF'
{"default":[{"exporterPhase":"EXPORTING","exportedPosition":1}],"tenanta":[],"tenantb":[{"exporterPhase":"EXPORTING","exportedPosition":3}]}
EOF

  [ "$status" -ne 0 ]
}

@test "token audience validation accepts only the intended Optimize audience" {
  token="x.$(encode_jwt_payload '{"aud":["optimize-orcha-tenanta-api","camunda-identity-resource-server"]}').x"
  run token_has_expected_audience "$token" optimize-orcha-tenanta-api optimize-orcha-tenantb-api
  [ "$status" -eq 0 ]

  run token_has_expected_audience "$token" optimize-orcha-tenantb-api optimize-orcha-tenanta-api
  [ "$status" -ne 0 ]

  token="x.$(encode_jwt_payload '{"aud":["optimize-orcha-tenanta-api","optimize-orcha-tenantb-api"]}').x"
  run token_has_expected_audience "$token" optimize-orcha-tenanta-api optimize-orcha-tenantb-api
  [ "$status" -ne 0 ]
}

@test "definition matching distinguishes own and sibling processes" {
  definitions='[{"key":"pt-accept-tenanta-1","name":"pt-accept-tenanta-1"}]'
  run definitions_contain "$definitions" pt-accept-tenanta-1
  [ "$status" -eq 0 ]
  run definitions_contain "$definitions" pt-accept-tenantb-1
  [ "$status" -ne 0 ]
}

@test "default and named tenants use the correct REST paths" {
  run tenant_api_path default
  [ "$output" = "/orchestration/v2" ]
  run tenant_api_path tenanta
  [ "$output" = "/orchestration/physical-tenants/tenanta/v2" ]
}
