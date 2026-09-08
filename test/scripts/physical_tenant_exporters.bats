#!/usr/bin/env bats

setup() {
  ROOT="$(git -C "$(dirname "${BATS_TEST_FILENAME}")" rev-parse --show-toplevel)"
  SCRIPT="$ROOT/charts/camunda-platform-8.10/test/integration/scenarios/pre-setup-scripts/post-deploy-physical-tenant-exporters.sh"
  eval "$(sed -n '/^partitions_are_healthy() {/,/^}/p' "$SCRIPT")"
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
