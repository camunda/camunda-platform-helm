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

installed_e2e_suite() {
  export INSTALLED_E2E_SUITE_VERSION="$1"
  npm() {
    printf '{"dependencies":{"@camunda/e2e-test-suite":{"version":"%s"}}}\n' "$INSTALLED_E2E_SUITE_VERSION"
  }
}

prebuilt_e2e_suite_dir() {
  local dir="$BATS_TEST_TMPDIR/e2e"
  mkdir -p "$dir/node_modules"
  printf '{"dependencies":{"@camunda/e2e-test-suite":"latest"}}\n' > "$dir/package.json"
  printf '%s\n' "$dir"
}

@test "a pinned e2e suite version is installed instead of latest" {
  npm() { printf 'npm %s\n' "$*"; }
  E2E_TEST_SUITE_VERSION=0.0.1251

  run _setup_playwright_environment "$(prebuilt_e2e_suite_dir)"

  [ "$status" -eq 0 ]
  [[ "$output" == *"npm install @camunda/e2e-test-suite@0.0.1251 --no-save"* ]]
}

@test "an unpinned run installs the latest e2e suite" {
  npm() { printf 'npm %s\n' "$*"; }
  unset E2E_TEST_SUITE_VERSION

  run _setup_playwright_environment "$(prebuilt_e2e_suite_dir)"

  [ "$status" -eq 0 ]
  [[ "$output" == *"npm install @camunda/e2e-test-suite@latest --no-save"* ]]
}

@test "an installed e2e suite that differs from the pinned version fails the run" {
  installed_e2e_suite 0.0.1250
  E2E_TEST_SUITE_VERSION=0.0.1251

  run _log_e2e_suite_version

  [ "$status" -eq 1 ]
  [[ "$output" == *"0.0.1250 does not match the pinned version 0.0.1251"* ]]
}

@test "an installed e2e suite that matches the pinned version passes" {
  installed_e2e_suite 0.0.1251
  E2E_TEST_SUITE_VERSION=0.0.1251

  run _log_e2e_suite_version

  [ "$status" -eq 0 ]
}

@test "an unpinned run accepts any installed e2e suite version" {
  installed_e2e_suite 0.0.1250
  unset E2E_TEST_SUITE_VERSION

  run _log_e2e_suite_version

  [ "$status" -eq 0 ]
}

@test "a locally linked e2e suite skips the pinned version check" {
  installed_e2e_suite 0.0.1
  E2E_TEST_SUITE_VERSION=0.0.1251
  PLAYWRIGHT_E2E_LOCAL_TEST_SUITE="$BATS_TEST_TMPDIR/local-suite"

  run _log_e2e_suite_version

  [ "$status" -eq 0 ]
}

slow_path_e2e_suite_dir() {
  local dir="$BATS_TEST_TMPDIR/e2e-slow"
  mkdir -p "$dir"
  printf '{"dependencies":{"@camunda/e2e-test-suite":"latest"}}\n' > "$dir/package.json"
  printf '%s\n' "$dir"
}

@test "the slow install path installs a pinned e2e suite version instead of updating to latest" {
  npm() { printf 'npm %s\n' "$*"; }
  PREBUILT_E2E_NODE_MODULES="$BATS_TEST_TMPDIR/no-prebuilt-tree"
  E2E_TEST_SUITE_VERSION=0.0.1251

  run _setup_playwright_environment "$(slow_path_e2e_suite_dir)"

  [[ "$output" == *"npm install @camunda/e2e-test-suite@0.0.1251 --save-exact"* ]]
  [[ "$output" != *"npm update"* ]]
}

@test "the slow install path updates an unpinned e2e suite to latest" {
  npm() { printf 'npm %s\n' "$*"; }
  PREBUILT_E2E_NODE_MODULES="$BATS_TEST_TMPDIR/no-prebuilt-tree"
  unset E2E_TEST_SUITE_VERSION

  run _setup_playwright_environment "$(slow_path_e2e_suite_dir)"

  [[ "$output" == *"npm update @camunda/e2e-test-suite"* ]]
}

stub_admin_role_check() {
  local script_dir="$BATS_TEST_TMPDIR/node_modules/@camunda/e2e-test-suite/scripts"
  mkdir -p "$script_dir"
  printf '#!/usr/bin/env bash\necho admin-role-check-ran\nexit %s\n' "$1" > "$script_dir/wait-for-orchestration-admin-role.sh"
}

@test "the api project runs the clock project too, on four workers, after the admin-role check" {
  stub_playwright_run
  stub_admin_role_check 0

  run run_playwright_tests "$BATS_TEST_TMPDIR" false 1 1 blob "" false false "" "" "" false api

  [ "$status" -eq 0 ]
  [[ "$output" == *"admin-role-check-ran"*"playwright-arg: --project=api"* ]]
  [[ "$output" == *"playwright-arg: --project=clock"* ]]
  [[ "$output" == *"playwright-arg: --workers=4"* ]]
}

@test "a failed admin-role check fails the api run" {
  stub_playwright_run
  stub_admin_role_check 1
  _handle_playwright_result() { echo "result: $1 $2"; }

  run run_playwright_tests "$BATS_TEST_TMPDIR" false 1 1 blob "" false false "" "" "" false api

  [[ "$output" == *"result: 1 REST v2 API suite admin-role readiness check"* ]]
}

@test "browser projects skip the admin-role check" {
  stub_playwright_run
  stub_admin_role_check 0

  run run_playwright_tests "$BATS_TEST_TMPDIR" false 1 1 blob "" false false "" "" "" false full-suite

  [ "$status" -eq 0 ]
  [[ "$output" != *"admin-role-check-ran"* ]]
  [[ "$output" != *"--project=clock"* ]]
}
