#!/usr/bin/env bats

setup() {
  if ROOT="$(git -C "$here" rev-parse --show-toplevel 2>/dev/null)"; then
    :
  else
    ROOT="$(cd "$here/../.." && pwd)"
  fi
  export ROOT
  export VERBOSE=false
  export TEST_INGRESS_HOST=""

  source "$ROOT/scripts/base_playwright_script.sh"
}

kubectl() {
  case "$*" in
    *"get ingress"*)
      printf '%s\n' '{"items":[]}'
      ;;
    *"get httproute"*)
      printf '%s\n' '{"items":[{"spec":{"hostnames":["camunda.example.com"]}}]}'
      ;;
    *"get gateway"*)
      printf '%s\n' '{"items":[{"spec":{"listeners":[{"hostname":"camunda.example.com"},{"hostname":"grpc-camunda.example.com"}]}}]}'
      ;;
    *)
      return 1
      ;;
  esac
}

@test "Gateway API hostname discovery uses the HTTPRoute host" {
  run get_ingress_hostname test-namespace

  [ "$status" -eq 0 ]
  [[ "$output" == *"camunda.example.com" ]]
  [[ "$output" != *"grpc-camunda.example.com"* ]]
  [ "${lines[${#lines[@]} - 1]}" = "camunda.example.com" ]
}

stub_playwright_run() {
  _setup_playwright_environment() { :; }
  _install_playwright_browsers() { :; }
  _log_e2e_suite_version() { :; }
  _run_playwright_with_retry() {
    shift 2
    printf 'playwright-arg: %s\n' "$@"
  }
  _handle_playwright_result() { :; }
}

@test "a single-shard Playwright run omits --shard so Playwright fails when no tests match" {
  stub_playwright_run

  run run_playwright_tests "$BATS_TEST_TMPDIR" false 1 1 blob "" false false "" "" "" false topology-orchestration

  [ "$status" -eq 0 ]
  [[ "$output" == *"playwright-arg: --project=topology-orchestration"* ]]
  [[ "$output" != *"--shard"* ]]
}

@test "a multi-shard Playwright run passes --shard" {
  stub_playwright_run

  run run_playwright_tests "$BATS_TEST_TMPDIR" false 2 3 blob "" false false

  [ "$status" -eq 0 ]
  [[ "$output" == *"playwright-arg: --shard=2/3"* ]]
}
