#!/usr/bin/env bats

setup() {
  if ROOT="$(git -C "$here" rev-parse --show-toplevel 2>/dev/null)"; then
    :
  else
    ROOT="$(cd "$here/../.." && pwd)"
  fi
  export ROOT
  export VERBOSE=false

  SUITE_DIR="$BATS_TEST_TMPDIR/suite"
  mkdir -p "$SUITE_DIR"
  printf 'PLAYWRIGHT_BASE_URL=https://${TEST_INGRESS_HOST}\n' > "$SUITE_DIR/.env.template"
  export SUITE_DIR

  export AUTH0_ISSUER_URL="https://tenant.example.auth0.com/"
  export AUTH0_AUDIENCE="camunda"
  export AUTH0_IDENTITY_CLIENT_ID="identity-client"
  export AUTH0_ORCHESTRATION_CLIENT_ID="orchestration-client"
  export AUTH0_OPTIMIZE_CLIENT_ID="optimize-client"
  export AUTH0_CONNECTORS_CLIENT_ID="connectors-client"
  export AUTH0_WEB_MODELER_CLIENT_ID="web-modeler-client"
  export AUTH0_CONSOLE_CLIENT_ID="console-client"

  # render-e2e-env.sh resolves its base-script path from $0, which is the bats
  # binary here; sourcing the base script first satisfies its `declare -f log` guard.
  source "$ROOT/scripts/base_playwright_script.sh"
  source "$ROOT/scripts/render-e2e-env.sh"
}

kubectl() {
  case "$*" in
    *"get pods"*)
      return 0
      ;;
    *"get secret"*)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

render_auth0_env() {
  render_env_file "$1" "$SUITE_DIR" "camunda.example.com" test-namespace \
    true false false false true "" true
}

@test "rendered env file is owner-readable only" {
  env_file="$BATS_TEST_TMPDIR/.env"

  run render_auth0_env "$env_file"

  [ "$status" -eq 0 ]
  [ -f "$env_file" ]
  [ "$(stat -f '%Lp' "$env_file" 2>/dev/null || stat -c '%a' "$env_file")" = "600" ]
  grep -q "PLAYWRIGHT_BASE_URL=https://camunda.example.com" "$env_file"
  grep -q "AUTH0_IDENTITY_CLIENT_ID=identity-client" "$env_file"
}

@test "rendered env file is tightened even when a world-readable file already exists" {
  env_file="$BATS_TEST_TMPDIR/.env"
  printf 'STALE=1\n' > "$env_file"
  chmod 644 "$env_file"

  run render_auth0_env "$env_file"

  [ "$status" -eq 0 ]
  [ "$(stat -f '%Lp' "$env_file" 2>/dev/null || stat -c '%a' "$env_file")" = "600" ]
  ! grep -q "STALE=1" "$env_file"
}

@test "writing the env file through a symbolic link is refused" {
  target="$BATS_TEST_TMPDIR/target"
  env_file="$BATS_TEST_TMPDIR/link"
  printf 'UNTOUCHED=1\n' > "$target"
  ln -s "$target" "$env_file"

  run render_auth0_env "$env_file"

  [ "$status" -eq 1 ]
  [[ "$output" == *"refusing to write env file through a symbolic link"* ]]
  grep -q "UNTOUCHED=1" "$target"
}

@test "env file contents are not echoed when verbose is enabled" {
  env_file="$BATS_TEST_TMPDIR/.env"
  export VERBOSE=true

  run render_auth0_env "$env_file"

  [ "$status" -eq 0 ]
  [[ "$output" == *"Env file written to $env_file"* ]]
  [[ "$output" != *"identity-client"* ]]
}
