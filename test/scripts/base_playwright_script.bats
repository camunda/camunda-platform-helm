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

@test "explicit project and file pattern are passed to Playwright" {
  _setup_playwright_environment() { :; }
  _install_playwright_browsers() { :; }
  _log_e2e_suite_version() { :; }
  _run_playwright_with_retry() {
    printf '%s\n' "$@"
  }
  _handle_playwright_result() { return "$1"; }

  run run_playwright_tests /tmp/e2e false 1 1 blob "" false false "" "" "rerun" false full-suite-v1 "tasklist/special flow/**/*[ab].spec.js"

  [ "$status" -eq 0 ]
  [[ "$output" == *"--project=full-suite-v1"* ]]
  [[ " ${lines[*]} " == *" tasklist/special flow/**/*[ab].spec.js "* ]]
  [[ "$output" != *"--pass-with-no-tests"* ]]
}

@test "zero-test Playwright failure is preserved" {
  _setup_playwright_environment() { :; }
  _install_playwright_browsers() { :; }
  _log_e2e_suite_version() { :; }
  _run_playwright_with_retry() { return 1; }
  _handle_playwright_result() { return "$1"; }

  run run_playwright_tests /tmp/e2e false 1 1 blob "" false false "" "" "rerun" false full-suite "missing/**/*.spec.js"

  [ "$status" -eq 1 ]
}
