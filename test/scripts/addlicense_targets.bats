#!/usr/bin/env bats

# Tests for the go.addlicense-* targets in the root Makefile.
# Stubs `addlicense` so the wiring can be exercised without Go installed:
# the stub records the argv it received and exits with a configurable code.
#
# These cover a regression where the targets silently passed for months:
# a `**` glob that /bin/sh never expanded, a doubled `charts/` prefix, and
# addlicense exiting 0 after failing to stat its argument.

setup() {
  here="$(cd "$(dirname "${BATS_TEST_FILENAME}")" && pwd)"
  if ROOT="$(git -C "$here" rev-parse --show-toplevel 2>/dev/null)"; then
    :
  else
    ROOT="$(cd "$here/../.." && pwd)"
  fi
  export ROOT

  TMPDIR_TEST="$(mktemp -d)"
  export TMPDIR_TEST
  export ARGV_LOG="$TMPDIR_TEST/argv.log"
  export PATH="$TMPDIR_TEST/bin:$PATH"
  mkdir -p "$TMPDIR_TEST/bin"
}

teardown() {
  rm -rf "$TMPDIR_TEST"
}

# Helper: install an addlicense shim that appends each path argument to
# ARGV_LOG and exits with $1.
install_addlicense_stub() {
  local exit_code="$1"
  cat > "$TMPDIR_TEST/bin/addlicense" <<STUBEOF
#!/usr/bin/env bash
for arg in "\$@"; do
  case "\$arg" in
    -*) ;;
    apache) ;;
    'Camunda Services GmbH') ;;
    *) echo "\$arg" >> "$ARGV_LOG" ;;
  esac
done
exit $exit_code
STUBEOF
  chmod +x "$TMPDIR_TEST/bin/addlicense"
}

@test "CI still invokes the license check" {
  # A working target is worthless if nothing calls it.
  run grep -rl 'go.addlicense-check' "$ROOT/.github/workflows"
  [ "$status" -eq 0 ]
  [ -n "$output" ]
}

@test "check passes a non-empty list of .go files to addlicense" {
  install_addlicense_stub 0

  run make -C "$ROOT" go.addlicense-check chartPath=charts/camunda-platform-8.10
  [ "$status" -eq 0 ]

  # The original bug: the glob never expanded, so addlicense received a
  # literal pattern and zero real files.
  [ -s "$ARGV_LOG" ]
  run grep -c '\.go$' "$ARGV_LOG"
  [ "$output" -gt 0 ]

  # Every argument must be a path that actually exists on disk.
  while read -r path; do
    [ -f "$ROOT/$path" ] || [ -f "$path" ]
  done < "$ARGV_LOG"
}

@test "check resolves chartPath given with the charts/ prefix" {
  install_addlicense_stub 0

  # chartPath already carries the charts/ prefix (Makefile) and AGENTS.md
  # documents invoking these targets with it; the recipe must not re-add it.
  run make -C "$ROOT" go.addlicense-check chartPath=charts/camunda-platform-8.10
  [ "$status" -eq 0 ]

  run grep -c '^charts/charts/' "$ARGV_LOG"
  [ "$output" -eq 0 ]

  run grep -c '^charts/camunda-platform-8\.10/test/' "$ARGV_LOG"
  [ "$output" -gt 0 ]
}

@test "check defaults to every chart version when chartPath is unset" {
  install_addlicense_stub 0

  # CI invokes `make go.addlicense-check` with no chartPath.
  run make -C "$ROOT" go.addlicense-check
  [ "$status" -eq 0 ]

  run cut -d/ -f2 "$ARGV_LOG"
  versions="$(echo "$output" | sort -u | wc -l)"
  [ "$versions" -gt 1 ]
}

@test "check fails when addlicense reports a missing header" {
  install_addlicense_stub 1

  # The core regression: a non-zero addlicense must fail the make target
  # rather than being swallowed by the pipeline.
  run make -C "$ROOT" go.addlicense-check chartPath=charts/camunda-platform-8.10
  [ "$status" -ne 0 ]

  # ...and it must have been handed real files, not an unexpanded pattern.
  run grep -c '\.go$' "$ARGV_LOG"
  [ "$output" -gt 0 ]
  run grep -c '\*' "$ARGV_LOG"
  [ "$output" -eq 0 ]
}

@test "check fails loudly when no .go files are found" {
  install_addlicense_stub 0

  # An empty file set must be an error, not a vacuous pass. BSD xargs skips
  # the command entirely on empty input and exits 0, so without the guard
  # this reports success while checking nothing.
  run make -C "$ROOT" go.addlicense-check chartPath=charts/does-not-exist
  [ "$status" -ne 0 ]
  [[ "$output" == *"no .go files under"* ]]

  # addlicense must never have been reached.
  [ ! -s "$ARGV_LOG" ] || [ ! -f "$ARGV_LOG" ]
}

@test "run fails loudly when no .go files are found" {
  install_addlicense_stub 0

  run make -C "$ROOT" go.addlicense-run chartPath=charts/does-not-exist
  [ "$status" -ne 0 ]
  [[ "$output" == *"no .go files under"* ]]
}

@test "run operates on the same real file set as check" {
  install_addlicense_stub 0

  run make -C "$ROOT" go.addlicense-run chartPath=charts/camunda-platform-8.10
  [ "$status" -eq 0 ]
  [ -s "$ARGV_LOG" ]

  run grep -c '\.go$' "$ARGV_LOG"
  [ "$output" -gt 0 ]
  run grep -c '\*' "$ARGV_LOG"
  [ "$output" -eq 0 ]

  # run and check must never drift apart in scope.
  cp "$ARGV_LOG" "$TMPDIR_TEST/run.log"
  rm -f "$ARGV_LOG"
  run make -C "$ROOT" go.addlicense-check chartPath=charts/camunda-platform-8.10
  [ "$status" -eq 0 ]
  run diff <(sort "$TMPDIR_TEST/run.log") <(sort "$ARGV_LOG")
  [ "$status" -eq 0 ]
}

@test "CI still invokes the license type verifier" {
  # go.license-verify covers what addlicense -check cannot: the license type.
  run grep -rl 'go.license-verify' "$ROOT/.github/workflows"
  [ "$status" -eq 0 ]
  [ -n "$output" ]
}

@test "the repo passes the license type verifier" {
  # Guards against a non-Apache header landing anywhere in the repo, which
  # addlicense -check accepts silently.
  if ! command -v go >/dev/null 2>&1; then
    skip "go not installed"
  fi
  run make -C "$ROOT" go.license-verify
  [ "$status" -eq 0 ]
  [[ "$output" == *"all Apache 2.0"* ]]
}
