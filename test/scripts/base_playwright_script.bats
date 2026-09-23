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

@test "Playwright invocation includes primary and additional projects" {
  _setup_playwright_environment() { :; }
  _install_playwright_browsers() { :; }
  _log_e2e_suite_version() { :; }
  _run_playwright_with_retry() {
    printf '%s\n' "$*"
    return 0
  }

  run run_playwright_tests /tmp false 1 1 blob "" false false "" "" rerun false primary additional

  [ "$status" -eq 0 ]
  [[ "$output" == *"--project=primary"* ]]
  [[ "$output" == *"--project=additional"* ]]
}
