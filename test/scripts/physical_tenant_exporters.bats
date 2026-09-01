#!/usr/bin/env bats

setup() {
  ROOT="$(git -C "$(dirname "${BATS_TEST_FILENAME}")" rev-parse --show-toplevel)"
  SCRIPT="$ROOT/charts/camunda-platform-8.10/test/integration/scenarios/pre-setup-scripts/post-deploy-physical-tenant-exporters.sh"
}

@test "physical tenant exporter hook only delegates to deploy-camunda" {
  run bash -n "$SCRIPT"
  [ "$status" -eq 0 ]

  run grep -c '^deploy-camunda "${args\[@\]}"' "$SCRIPT"
  [ "$status" -eq 0 ]
  [ "$output" -eq 1 ]

  run grep -E 'curl|kubectl|jq|base64' "$SCRIPT"
  [ "$status" -ne 0 ]
}
