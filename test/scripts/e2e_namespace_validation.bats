#!/usr/bin/env bats

setup() {
  if ROOT="$(git -C "$here" rev-parse --show-toplevel 2>/dev/null)"; then
    :
  else
    ROOT="$(cd "$here/../.." && pwd)"
  fi
  export ROOT

  log() {
    :
  }
  source "$ROOT/scripts/render-e2e-env.sh"
}

@test "namespace validation accepts an existing namespace" {
  kubectl() {
    printf '%s\n' 'namespace/test-namespace'
  }

  run validate_namespace_access test-namespace

  [ "$status" -eq 0 ]
}

@test "namespace validation reports a missing namespace" {
  kubectl() {
    return 0
  }

  run validate_namespace_access test-namespace

  [ "$status" -eq 1 ]
  [[ "$output" == *"namespace 'test-namespace' not found"* ]]
}

@test "namespace validation preserves API failures" {
  kubectl() {
    printf '%s\n' 'Unable to connect to the server: token expired' >&2
    return 1
  }

  run validate_namespace_access test-namespace test-context

  [ "$status" -eq 1 ]
  [[ "$output" == *"cannot query the Kubernetes API"* ]]
  [[ "$output" == *"token expired"* ]]
  [[ "$output" != *"namespace 'test-namespace' not found"* ]]
}
