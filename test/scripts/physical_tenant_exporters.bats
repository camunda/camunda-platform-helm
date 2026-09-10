#!/usr/bin/env bats

setup() {
  here="$(cd "$(dirname "${BATS_TEST_FILENAME}")" && pwd)"
  if ROOT="$(git -C "$here" rev-parse --show-toplevel 2>/dev/null)"; then
    :
  else
    ROOT="$(cd "$here/../.." && pwd)"
  fi
  SCRIPT="$ROOT/charts/camunda-platform-8.10/test/integration/scenarios/pre-setup-scripts/post-deploy-physical-tenant-exporters.sh"
  eval "$(sed -n '/^partitions_are_healthy() {/,/^}/p' "$SCRIPT")"
}

# Fixtures mirror /orchestration/actuator/partitions: each tenant maps to an object keyed by
# partition id, not to an array.

@test "partition validation accepts the complete healthy tenant set" {
  run partitions_are_healthy <<'EOF'
{"default":{"1":{"exporterPhase":"EXPORTING","exportedPosition":908}},"tenanta":{"1":{"exporterPhase":"EXPORTING","exportedPosition":164}},"tenantb":{"1":{"exporterPhase":"EXPORTING","exportedPosition":164}}}
EOF

  [ "$status" -eq 0 ]
}

@test "partition validation accepts a tenant with several partitions" {
  run partitions_are_healthy <<'EOF'
{"default":{"1":{"exporterPhase":"EXPORTING","exportedPosition":908},"2":{"exporterPhase":"EXPORTING","exportedPosition":12}},"tenanta":{"1":{"exporterPhase":"EXPORTING","exportedPosition":164}},"tenantb":{"1":{"exporterPhase":"EXPORTING","exportedPosition":164}}}
EOF

  [ "$status" -eq 0 ]
}

@test "partition validation rejects omission of either configured tenant" {
  run partitions_are_healthy <<'EOF'
{"default":{"1":{"exporterPhase":"EXPORTING","exportedPosition":908}},"tenanta":{"1":{"exporterPhase":"EXPORTING","exportedPosition":164}}}
EOF
  [ "$status" -ne 0 ]

  run partitions_are_healthy <<'EOF'
{"default":{"1":{"exporterPhase":"EXPORTING","exportedPosition":908}},"tenantb":{"1":{"exporterPhase":"EXPORTING","exportedPosition":164}}}
EOF
  [ "$status" -ne 0 ]
}

@test "partition validation rejects a tenant without partitions" {
  run partitions_are_healthy <<'EOF'
{"default":{"1":{"exporterPhase":"EXPORTING","exportedPosition":908}},"tenanta":{},"tenantb":{"1":{"exporterPhase":"EXPORTING","exportedPosition":164}}}
EOF

  [ "$status" -ne 0 ]
}

@test "partition validation rejects a tenant that is not exporting" {
  run partitions_are_healthy <<'EOF'
{"default":{"1":{"exporterPhase":"EXPORTING","exportedPosition":908}},"tenanta":{"1":{"exporterPhase":"EXPORTING","exportedPosition":164}},"tenantb":{"1":{"exporterPhase":"PAUSED","exportedPosition":164}}}
EOF

  [ "$status" -ne 0 ]
}

# A BlockingExporter keeps the phase at EXPORTING while the position stays at -1.
@test "partition validation rejects a blocked exporter still reporting EXPORTING" {
  run partitions_are_healthy <<'EOF'
{"default":{"1":{"exporterPhase":"EXPORTING","exportedPosition":908}},"tenanta":{"1":{"exporterPhase":"EXPORTING","exportedPosition":164}},"tenantb":{"1":{"exporterPhase":"EXPORTING","exportedPosition":-1}}}
EOF

  [ "$status" -ne 0 ]
}
