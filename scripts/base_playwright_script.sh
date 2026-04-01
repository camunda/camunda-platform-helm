#!/usr/bin/env bash

# ==============================================================================
# Camunda Platform – Integration/e2e-Test Runner
# ------------------------------------------------------------------------------
# Why does this script exist?
#   *  A single, developer-friendly entry-point for running the Playwright-based
#      integration test-suite that lives under <chart>/test/integration or /test/e2e.
#   *  Works both locally on a developer laptop **and** inside GitHub Actions
#      without modification.
#   *  Hardened: performs extensive sanity-checks, validates prerequisites and
#      cleans up after itself so CI troubleshooting is painless.
#
# What does it actually do?
#   1. Verifies required CLI tools are available (kubectl, jq, git, npm, …).
#   2. Validates the supplied Helm chart path and Kubernetes namespace.
#   3. Detects the ingress hostname for the Camunda Platform installation and
#      exports it for the tests as TEST_INGRESS_HOST.
#   4. Builds a temporary .env file populated with service client secrets and
#      Playwright variables, removing it automatically on exit.
#   5. Installs Node dependencies with `npm install` and finally executes the
#      Playwright test runner.
#
# Expected environment / assumptions
#   • kubectl context points at a cluster where the Camunda Platform Helm chart
#     is already installed in the provided namespace.
#   • A secret named `integration-test-credentials` exists in that namespace
#
# Usage examples
#   # Local run against KIND cluster
#   ./scripts/run-integration-tests.sh \
#       --chart-path /home/runner/work/camunda-platform-helm/charts/camunda-platform-8.7 \
#       --namespace camunda
#
#   ./scripts/run-e2e-tests.sh \
#       --chart-path /home/runner/work/camunda-platform-helm/charts/camunda-platform-8.7 \
#       --namespace camunda
#
# Any failure will terminate the script with a non-zero exit code so that CI
# systems mark the job as failed.
# ============================================================================

# Color definitions — disabled when stderr is not a terminal (e.g., redirected to a log file)
if [[ -t 2 ]]; then
  COLOR_RESET='\033[0m'
  COLOR_RED='\033[0;31m'
  COLOR_GREEN='\033[0;32m'
  COLOR_YELLOW='\033[0;33m'
  COLOR_BLUE='\033[0;34m'
  COLOR_MAGENTA='\033[0;35m'
  COLOR_CYAN='\033[0;36m'
  COLOR_GRAY='\033[0;90m'
else
  COLOR_RESET=''
  COLOR_RED=''
  COLOR_GREEN=''
  COLOR_YELLOW=''
  COLOR_BLUE=''
  COLOR_MAGENTA=''
  COLOR_CYAN=''
  COLOR_GRAY=''
fi

# Always-visible status output for long-running steps.
# Use this instead of log() for messages the user must see regardless of -v.
info() {
  echo -e "${COLOR_CYAN}[$(date +'%H:%M:%S')]${COLOR_RESET} $*" >&2
}

log() {
  if $VERBOSE; then
    local message="$*"
    local color="$COLOR_RESET"
    
    # Color based on message type
    if [[ "$message" == *"ERROR"* ]] || [[ "$message" == *"Error"* ]] || [[ "$message" == "❌"* ]]; then
      color="$COLOR_RED"
    elif [[ "$message" == "✅"* ]]; then
      color="$COLOR_GREEN"
    elif [[ "$message" == "DEBUG:"* ]]; then
      color="$COLOR_GRAY"
    elif [[ "$message" == *"WARNING"* ]] || [[ "$message" == *"Warning"* ]]; then
      color="$COLOR_YELLOW"
    fi
    
    echo -e "${color}[$(date +'%Y-%m-%dT%H:%M:%S%z')]: $message${COLOR_RESET}" >&2
  fi
}

get_ingress_hostname() {
  local namespace="$1"
  local kube_context="${2:-}"
  local hostname
  local kubectl_cmd="kubectl"

  if [[ -n "$TEST_INGRESS_HOST" ]]; then
    echo "$TEST_INGRESS_HOST"
    return 0
  fi
  
  if [[ -n "$kube_context" ]]; then
    kubectl_cmd="kubectl --context=$kube_context"
  fi

  info "Detecting ingress hostname in namespace ${namespace}..."
  hostname=$($kubectl_cmd -n "$namespace" get ingress -o json | jq -r '
    .items[]
    | select(all(.spec.rules[].host; (contains("zeebe") or contains("grpc")) | not))
    | ([.spec.rules[].host] | join(","))')
  if [[ -z "$hostname" ]]; then
    # might be using the Gateway api
    log "No matching Ingress found, trying Gateway API..."
    hostname=$($kubectl_cmd -n "$namespace" get gateway -o json | jq -r '.items[].spec.listeners[].hostname')
  fi

  if [[ -z "$hostname" || "$hostname" == "null" ]]; then
    echo "Error: unable to determine ingress hostname in namespace '$namespace'" >&2
    exit 1
  fi

  info "Ingress hostname: ${hostname}"
  echo "$hostname"
}

check_required_cmds() {
  required_cmds=(kubectl jq git envsubst npm npx curl)
  for cmd in "${required_cmds[@]}"; do
    if ! command -v "$cmd" >/dev/null 2>&1; then
      echo "Error: required command '$cmd' not found in PATH" >&2
      exit 127
    fi
  done
}

# Resolve a hostname to an IP address, trying the system resolver first,
# falling back to public DNS servers, and finally querying authoritative
# nameservers directly (which bypasses negative-cache TTLs entirely).
# Prints the resolved IP on stdout.
# Returns 0 on success, 1 if no resolver can find the host.
_resolve_host() {
  local hostname="$1"
  local ip

  # Try system resolver
  ip=$(nslookup "$hostname" 2>/dev/null | awk '/^Address: / { print $2; exit }')
  if [[ -n "$ip" ]]; then
    echo "$ip"
    return 0
  fi

  # Try multiple public DNS servers — any single one may have a stale NXDOMAIN
  # cached (the SOA negative-cache TTL for this zone is 300s).
  local resolver
  for resolver in 1.1.1.1 8.8.8.8 9.9.9.9; do
    ip=$(nslookup "$hostname" "$resolver" 2>/dev/null | awk '/^Address: / { a=$2 } END { print a }')
    if [[ -n "$ip" ]]; then
      echo "$ip"
      return 0
    fi
  done

  # All recursive resolvers failed — they likely have a stale NXDOMAIN cached.
  # Query the authoritative nameservers directly to bypass negative caching.
  ip=$(_resolve_host_authoritative "$hostname")
  if [[ -n "$ip" ]]; then
    echo "$ip"
    return 0
  fi

  return 1
}

# Query authoritative nameservers for a hostname, bypassing recursive caches.
# Walks up the domain hierarchy to find the NS records for the zone, then
# queries those nameservers directly.  This is the last-resort fallback when
# all recursive resolvers have a stale NXDOMAIN cached.
_resolve_host_authoritative() {
  local hostname="$1"
  local domain="$hostname"
  local ns_servers=""

  # Walk up the domain hierarchy to find authoritative NS records.
  # e.g., for "foo.ci.distro.ultrawombat.com" try:
  #   ci.distro.ultrawombat.com → distro.ultrawombat.com → ultrawombat.com
  while [[ "$domain" == *.* ]]; do
    domain="${domain#*.}"
    ns_servers=$(nslookup -type=ns "$domain" 8.8.8.8 2>/dev/null \
      | awk '/nameserver =/ { print $NF }' | sed 's/\.$//')
    if [[ -n "$ns_servers" ]]; then
      break
    fi
  done

  if [[ -z "$ns_servers" ]]; then
    return 1
  fi

  # Query each authoritative nameserver directly (no cache to poison).
  local ns ip
  for ns in $ns_servers; do
    ip=$(nslookup "$hostname" "$ns" 2>/dev/null | awk '/^Address: / { a=$2 } END { print a }')
    if [[ -n "$ip" ]]; then
      echo "$ip"
      return 0
    fi
  done

  return 1
}

# ── DNS fallback for Node.js/Playwright ──
# When the system resolver has a stale NXDOMAIN but public DNS can resolve the
# host, we inject a Node.js preload script via NODE_OPTIONS that monkey-patches
# dns.lookup to fall back to public resolvers (1.1.1.1, 8.8.8.8, 9.9.9.9).
# Additionally, when a hostname and resolved IP are available (from
# _wait_for_dns_resolution), we pass them as DNS_FALLBACK_MAP so the Node.js
# script can resolve them instantly without any network calls.
# This avoids needing sudo or /etc/hosts edits.
_DNS_FALLBACK_ENABLED=false

# Usage: _enable_dns_fallback [hostname] [resolved_ip]
# When hostname and resolved_ip are provided, the Node.js DNS fallback script
# will use a static hostname→IP mapping for instant resolution.
# Common hostname variants (grpc-*, zeebe-*) are automatically included.
_enable_dns_fallback() {
  local hostname="${1:-}"
  local resolved_ip="${2:-}"
  local fallback_script
  fallback_script="$(dirname "${BASH_SOURCE[0]}")/dns-fallback.cjs"

  if [[ ! -f "$fallback_script" ]]; then
    info "${COLOR_YELLOW}WARNING:${COLOR_RESET} dns-fallback.cjs not found at ${fallback_script}. Playwright may fail with ENOTFOUND."
    return 1
  fi

  # Use absolute path so it works regardless of cwd
  fallback_script="$(cd "$(dirname "$fallback_script")" && pwd)/$(basename "$fallback_script")"

  # Append to NODE_OPTIONS so we don't clobber existing values
  export NODE_OPTIONS="${NODE_OPTIONS:+${NODE_OPTIONS} }--require ${fallback_script}"

  # When we have a pre-resolved hostname→IP, export a static mapping so the
  # Node.js fallback can resolve it instantly without network calls.
  # Include common prefixed variants (grpc-*, zeebe-*) that share the same IP.
  if [[ -n "$hostname" && -n "$resolved_ip" ]]; then
    local map_entries="${hostname}=${resolved_ip}"
    map_entries+=",grpc-${hostname}=${resolved_ip}"
    map_entries+=",zeebe-${hostname}=${resolved_ip}"
    export DNS_FALLBACK_MAP="$map_entries"
    info "Enabled Node.js DNS fallback with static map: ${hostname} -> ${resolved_ip} (+ grpc-/zeebe- variants)"
  else
    info "Enabled Node.js DNS fallback (public resolvers only) via NODE_OPTIONS"
  fi

  _DNS_FALLBACK_ENABLED=true
}

# Wait for DNS resolution of a hostname before proceeding.
# External-dns can take time to create records after ingress creation,
# and DNS propagation adds additional delay.
# Checks both the system resolver and multiple public DNS servers to avoid
# getting stuck on stale negative (NXDOMAIN) caches.
# On success, sets _RESOLVED_IP so callers (e.g. curl) can use --resolve to
# bypass a broken local cache.  When the system resolver is stale, sets
# _NEEDS_DNS_FALLBACK=true so callers can enable the Node.js DNS preload.
# Args: hostname, [timeout_seconds=120]
_RESOLVED_IP=""
_NEEDS_DNS_FALLBACK=false
_wait_for_dns_resolution() {
  local hostname="$1"
  local timeout="${2:-120}"
  local elapsed=0
  _RESOLVED_IP=""
  _NEEDS_DNS_FALLBACK=false

  # Skip if hostname is an IP address (IPv4 or IPv6)
  if [[ "$hostname" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]] || [[ "$hostname" == *:* ]]; then
    _RESOLVED_IP="$hostname"
    return 0
  fi

  info "Waiting for DNS resolution of ${hostname} (timeout ${timeout}s)..."

  while true; do
    local ip
    ip=$(_resolve_host "$hostname") && {
      _RESOLVED_IP="$ip"
      # Check whether the system resolver can resolve (not just public DNS)
      if ! nslookup "$hostname" >/dev/null 2>&1; then
        # Public DNS resolved but local resolver has stale NXDOMAIN.
        # Wait for the local negative cache to expire (SOA minimum = 300s)
        # so that non-Node.js tools (zbctl, curl without --resolve, etc.)
        # can resolve the hostname via the normal OS resolver.
        info "${COLOR_YELLOW}WARNING:${COLOR_RESET} Resolved via public DNS (-> ${ip}) but local resolver still returns NXDOMAIN."
        info "  Flushing local DNS cache and waiting for system resolver (negative TTL ≤ 300s)..."

        # Flush the macOS DNS cache (no sudo needed)
        if command -v dscacheutil >/dev/null 2>&1; then
          dscacheutil -flushcache 2>/dev/null || true
        fi

        # Poll the system resolver up to 360s (300s SOA minimum + 60s buffer).
        # Check every 10s to balance responsiveness vs noise.
        local local_timeout=360
        local local_elapsed=0
        local local_resolved=false
        while [[ $local_elapsed -lt $local_timeout ]]; do
          if nslookup "$hostname" >/dev/null 2>&1; then
            local_resolved=true
            info "System DNS resolver caught up after ${local_elapsed}s"
            break
          fi
          sleep 10
          local_elapsed=$((local_elapsed + 10))
          # Re-flush every 60s in case the local cache re-populated
          if (( local_elapsed % 60 == 0 )); then
            if command -v dscacheutil >/dev/null 2>&1; then
              dscacheutil -flushcache 2>/dev/null || true
            fi
            info "  Still waiting for system resolver (${local_elapsed}s/${local_timeout}s)..."
          fi
        done

        if [[ "$local_resolved" != "true" ]]; then
          _NEEDS_DNS_FALLBACK=true
          info "${COLOR_YELLOW}WARNING:${COLOR_RESET} System resolver did not catch up after ${local_timeout}s."
          info "  Will inject Node.js DNS fallback for Playwright (non-Node tools like zbctl may still fail)."
        fi
      fi
      break
    }

    if [[ $elapsed -ge $timeout ]]; then
      info "${COLOR_RED}ERROR:${COLOR_RESET} DNS resolution for '${hostname}' timed out after ${timeout}s (checked system + public DNS)"
      return 1
    fi
    sleep 5
    elapsed=$((elapsed + 5))
    info "  DNS not yet resolved (${elapsed}s/${timeout}s)..."
  done

  info "DNS resolved: ${hostname} -> ${_RESOLVED_IP} (${elapsed}s)"
}

# Wait for ingress paths to return non-502/503/504 responses.
# After DNS resolves, cloud load balancers (GKE NEG, EKS ALB, etc.) may still
# need time to converge on backend health state. This function polls each
# ingress context path until the LB routes traffic end-to-end.
# Uses _RESOLVED_IP (set by _wait_for_dns_resolution) to add a curl --resolve
# flag when the system resolver cannot resolve the hostname (stale NXDOMAIN).
# Args: hostname, namespace, [timeout_seconds=120], [kube_context]
_wait_for_ingress_ready() {
  local hostname="$1"
  local namespace="$2"
  local timeout="${3:-120}"
  local kube_context="${4:-}"
  local kubectl_cmd="kubectl"

  if [[ -n "$kube_context" ]]; then
    kubectl_cmd="kubectl --context=$kube_context"
  fi

  # Build --resolve flag for curl when the system resolver can't reach the host
  local resolve_flag=()
  if [[ -n "${_RESOLVED_IP:-}" ]] && ! nslookup "$hostname" >/dev/null 2>&1; then
    resolve_flag=(--resolve "${hostname}:443:${_RESOLVED_IP}")
    log "Using curl --resolve ${hostname}:443:${_RESOLVED_IP} (system DNS is stale)"
  fi

  # Extract unique context paths from ingress objects (filter out zeebe/grpc, same as get_ingress_hostname)
  local paths
  paths=$($kubectl_cmd -n "$namespace" get ingress -o json 2>/dev/null | jq -r '
    [.items[]
     | select(all(.spec.rules[].host; (contains("zeebe") or contains("grpc")) | not))
     | .spec.rules[].http.paths[].path]
    | map(ltrimstr("/") | split("/")[0] | "/" + .)
    | unique
    | .[]' 2>/dev/null)

  # Fall back to HTTPRoute (Gateway API) if no ingress paths found
  if [[ -z "$paths" ]]; then
    paths=$($kubectl_cmd -n "$namespace" get httproute -o json 2>/dev/null | jq -r '
      [.items[].spec.rules[].matches[]?.path.value // empty]
      | map(ltrimstr("/") | split("/")[0] | "/" + .)
      | unique
      | .[]' 2>/dev/null)
  fi

  if [[ -z "$paths" ]]; then
    info "${COLOR_YELLOW}WARNING:${COLOR_RESET} No ingress/httproute paths found in namespace '${namespace}', skipping readiness check"
    return 0
  fi

  info "Waiting for ingress to become ready on ${hostname} (timeout ${timeout}s)..."
  info "  Paths: $(echo "$paths" | tr '\n' ' ')"

  local elapsed=0
  while [[ $elapsed -lt $timeout ]]; do
    local all_ready=true
    local status_summary=""

    while IFS= read -r path; do
      [[ -z "$path" ]] && continue
      local http_code
      http_code=$(curl -sk -o /dev/null -w '%{http_code}' --connect-timeout 5 --max-time 10 "${resolve_flag[@]}" "https://${hostname}${path}" 2>/dev/null || echo "000")
      status_summary+=" ${path}=${http_code}"

      # 502, 503, 504, 000 (connection failure) mean the LB is not ready
      if [[ "$http_code" == "502" || "$http_code" == "503" || "$http_code" == "504" || "$http_code" == "000" ]]; then
        all_ready=false
      fi
    done <<< "$paths"

    if [[ "$all_ready" == "true" ]]; then
      info "Ingress ready (${elapsed}s):${status_summary}"
      return 0
    fi

    info "  Not ready yet (${elapsed}s/${timeout}s):${status_summary}"
    sleep 5
    elapsed=$((elapsed + 5))
  done

  info "${COLOR_RED}ERROR:${COLOR_RESET} Ingress readiness timed out after ${timeout}s on ${hostname}"
  return 1
}

# ==============================================================================
# Playwright Helper Functions
# ==============================================================================

# Portable file hash — abstracts md5sum (Linux) vs md5 (macOS) vs shasum (fallback)
_portable_file_hash() {
  local file="$1"
  if command -v md5sum >/dev/null 2>&1; then
    md5sum "$file" | cut -d' ' -f1
  elif command -v md5 >/dev/null 2>&1; then
    md5 -q "$file"
  elif command -v shasum >/dev/null 2>&1; then
    shasum "$file" | cut -d' ' -f1
  else
    echo "none"
  fi
}

# Check if node_modules matches the current package-lock.json hash
_is_node_modules_current() {
  [[ -d "node_modules" ]] && [[ -f "package-lock.json" ]] && [[ -f "node_modules/.package-lock-hash" ]] || return 1
  local current_hash cached_hash
  current_hash=$(_portable_file_hash "package-lock.json")
  cached_hash=$(cat "node_modules/.package-lock-hash" 2>/dev/null || echo "")
  [[ -n "$current_hash" ]] && [[ "$current_hash" != "none" ]] && [[ "$current_hash" == "$cached_hash" ]] || return 1

  # Hash match alone is not enough — node_modules can be corrupt (e.g., a prior
  # npm install was interrupted leaving only a fraction of files).  Verify that
  # the key Playwright module can actually be loaded before trusting the cache.
  # When corruption is detected we must remove node_modules entirely — npm
  # trusts its own internal manifest (.package-lock.json) and will report
  # "up to date" even when packages are incomplete.
  if ! node -e "require('@playwright/test')" 2>/dev/null; then
    log "node_modules hash matches but @playwright/test cannot be loaded — nuking node_modules to force clean reinstall"
    rm -rf "node_modules"
    return 1
  fi
}

# mkdir-based mutual exclusion for npm install (POSIX-portable, no flock dependency)
# Returns 0 if lock acquired, 1 on timeout (caller should proceed anyway)
_acquire_npm_lock() {
  local base_dir="$1"
  local timeout="${2:-120}"
  local lock_dir="${base_dir}/.npm-install-lock"
  local elapsed=0

  while ! mkdir "$lock_dir" 2>/dev/null; do
    # Check for stale lock — owner PID no longer running
    local lock_pid
    lock_pid=$(cat "$lock_dir/pid" 2>/dev/null || echo "")
    if [[ -n "$lock_pid" ]] && ! kill -0 "$lock_pid" 2>/dev/null; then
      log "Stale npm install lock (PID $lock_pid is dead), removing"
      rm -rf "$lock_dir"
      continue
    fi

    # Age-based fallback for SIGKILL / zombie cases (>300s = stale)
    if [[ -f "$lock_dir/pid" ]]; then
      local lock_age
      # stat -f %m = macOS, stat -c %Y = Linux
      lock_age=$(( $(date +%s) - $(stat -f %m "$lock_dir/pid" 2>/dev/null || stat -c %Y "$lock_dir/pid" 2>/dev/null || echo "0") ))
      if [[ $lock_age -gt 300 ]]; then
        log "npm install lock is ${lock_age}s old (>300s), treating as stale"
        rm -rf "$lock_dir"
        continue
      fi
    fi

    if [[ $elapsed -ge $timeout ]]; then
      log "WARNING: Timed out waiting for npm install lock after ${timeout}s, proceeding anyway"
      return 1
    fi

    log "Waiting for npm install lock (held by PID ${lock_pid:-unknown}, ${elapsed}s/${timeout}s)..."
    sleep 2
    elapsed=$((elapsed + 2))
  done

  echo $$ > "$lock_dir/pid"
  log "Acquired npm install lock (PID $$)"
  return 0
}

_release_npm_lock() {
  local base_dir="$1"
  local lock_dir="${base_dir}/.npm-install-lock"
  rm -rf "$lock_dir"
  log "Released npm install lock"
}

# Replace the npm-installed @camunda/e2e-test-suite with a copy of the local
# checkout's dist/ so Playwright resolves test files from within the e2e
# node_modules tree (avoiding a second @playwright/test from the local
# checkout's own node_modules).
# The local checkout must have been built (npm run build) so dist/ exists.
# Args: test_suite_path, local_dir
_link_local_test_suite() {
  local test_suite_path="$1"
  local local_dir="$2"

  local_dir="${local_dir%/}"

  if [[ ! -d "$local_dir" ]]; then
    echo "Error: --local-test-suite directory does not exist: $local_dir" >&2
    exit 1
  fi

  if [[ ! -f "$local_dir/package.json" ]]; then
    echo "Error: --local-test-suite directory has no package.json: $local_dir" >&2
    exit 1
  fi

  local pkg_name
  pkg_name=$(node -p "require('$local_dir/package.json').name" 2>/dev/null)
  if [[ "$pkg_name" != "@camunda/e2e-test-suite" ]]; then
    echo "Error: --local-test-suite package name is '$pkg_name', expected '@camunda/e2e-test-suite'" >&2
    exit 1
  fi

  if [[ ! -d "$local_dir/dist" ]]; then
    echo "Error: --local-test-suite has no dist/ directory — run 'npm run build' in $local_dir first" >&2
    exit 1
  fi

  local target="$test_suite_path/node_modules/@camunda/e2e-test-suite"
  rm -rf "$target"
  mkdir -p "$target"
  cp "$local_dir/package.json" "$target/package.json"
  cp -R "$local_dir/dist" "$target/dist"
  info "Copied local test suite dist into $target from $local_dir"
}

# Log the installed @camunda/e2e-test-suite version for debugging.
# Must be called after _setup_playwright_environment (which cd's into the test suite
# directory and runs npm install).  Uses a guard variable so the version is only
# printed once per shell invocation even when multiple entrypoints call this.
_log_e2e_suite_version() {
  if [[ -n "${_E2E_SUITE_VERSION_LOGGED:-}" ]]; then
    return
  fi
  _E2E_SUITE_VERSION_LOGGED=true

  local version
  version=$(npm ls @camunda/e2e-test-suite --json 2>/dev/null | jq -r '.dependencies["@camunda/e2e-test-suite"].version // "unknown"') || version="unknown"
  info "E2E test suite version: ${version}"
}

# Setup playwright environment: change directory, install dependencies, create test-results dir
# Uses double-checked locking to prevent concurrent npm install corruption.
# Before installing, updates @camunda/e2e-test-suite to the latest version from
# the registry so that the "latest" tag in package.json is actually resolved
# (npm install alone never upgrades past the version pinned in package-lock.json).
#
# When PLAYWRIGHT_E2E_LOCAL_TEST_SUITE is set, symlinks the local checkout into
# node_modules instead of using the npm-published package. This allows iterating
# on the test suite source directly.  The SM-8.7 ModelerHomePage patch is skipped
# in this mode because the user can modify their local source.
# Args: test_suite_path, [silent=false]
_setup_playwright_environment() {
  local test_suite_path="$1"
  local silent="${2:-false}"

  log "Changing directory to $test_suite_path"
  cd "$test_suite_path" || exit

  local npm_flags="--no-audit --no-fund"
  if [[ "$silent" == "true" ]]; then
    npm_flags="$npm_flags --silent"
  fi

  # Acquire the lock first — npm update and npm install both modify
  # node_modules and package-lock.json, so they must be serialized.
  local got_lock=true
  _acquire_npm_lock "$test_suite_path" 120 || got_lock=false

  # Update @camunda/e2e-test-suite to the latest version from the registry.
  # npm install respects package-lock.json and never upgrades past the pinned
  # version, so without this step the "latest" tag in package.json is ignored
  # once a lock file exists.  npm update rewrites the lock file when a newer
  # version is available, which also invalidates the hash cache below.
  local suite_updated=false
  if [[ -f "package.json" ]] && grep -q '@camunda/e2e-test-suite' package.json 2>/dev/null; then
    local pre_hash=""
    [[ -f "package-lock.json" ]] && pre_hash=$(_portable_file_hash "package-lock.json")
    info "Checking for newer @camunda/e2e-test-suite..."
    # shellcheck disable=SC2086
    npm update @camunda/e2e-test-suite $npm_flags 2>/dev/null || true
    local post_hash=""
    [[ -f "package-lock.json" ]] && post_hash=$(_portable_file_hash "package-lock.json")
    if [[ "$pre_hash" != "$post_hash" ]]; then
      suite_updated=true
      info "@camunda/e2e-test-suite was updated — will run full npm install to reconcile dependencies"
    fi
  fi

  # If the test suite was updated, always run a full npm install to reconcile
  # the entire dependency tree (npm update only touches the named package and
  # can leave transitive deps like playwright in an inconsistent state).
  # Otherwise, check whether node_modules already matches the lock file.
  if [[ "$suite_updated" != "true" ]] && _is_node_modules_current; then
    log "node_modules is up to date, skipping npm install"
    [[ "$got_lock" == "true" ]] && _release_npm_lock "$test_suite_path"
    if [[ -n "${PLAYWRIGHT_E2E_LOCAL_TEST_SUITE:-}" ]]; then
      _link_local_test_suite "$test_suite_path" "$PLAYWRIGHT_E2E_LOCAL_TEST_SUITE"
    fi
    local results_dir="${PLAYWRIGHT_TEST_OUTPUT:-$test_suite_path/test-results}"
    mkdir -p "$results_dir"
    return 0
  fi

  info "Installing npm dependencies..."
  # shellcheck disable=SC2086
  if npm install $npm_flags; then
    # Store hash only on success — a partial/failed install must not be cached
    # or future runs will skip reinstallation and inherit the broken state.
    _portable_file_hash "package-lock.json" > node_modules/.package-lock-hash || true
  else
    info "npm install failed — removing hash cache to force reinstall on next run"
    rm -f "node_modules/.package-lock-hash"
  fi

  [[ "$got_lock" == "true" ]] && _release_npm_lock "$test_suite_path"

  if [[ -n "${PLAYWRIGHT_E2E_LOCAL_TEST_SUITE:-}" ]]; then
    _link_local_test_suite "$test_suite_path" "$PLAYWRIGHT_E2E_LOCAL_TEST_SUITE"
  fi

  # Create the test-results directory; use namespace-scoped path when set.
  local results_dir="${PLAYWRIGHT_TEST_OUTPUT:-$test_suite_path/test-results}"
  mkdir -p "$results_dir"
}

# Install Playwright browsers (with deps on Linux)
# Skips installation if browsers are already present (e.g., in pre-built container image)
_install_playwright_browsers() {
  # Check if we're running in a container with pre-installed browsers
  # The official Playwright Docker image sets PLAYWRIGHT_BROWSERS_PATH
  # TODO: fix if statement proper conditional.
  # if [[ -n "${PLAYWRIGHT_BROWSERS_PATH:-}" ]] && [[ -d "${PLAYWRIGHT_BROWSERS_PATH}" ]]; then
  #   local browser_count
  #   browser_count=$(find "${PLAYWRIGHT_BROWSERS_PATH}" -maxdepth 1 -type d | wc -l)
  #   if [[ "$browser_count" -gt 1 ]]; then
  #     log "Playwright browsers already installed at ${PLAYWRIGHT_BROWSERS_PATH}, skipping installation"
  #     return 0
  #   fi
  # fi

  # Also check common Playwright browser locations
  # TODO: fix if statement proper conditional.
  # local ms_playwright_path="/ms-playwright"
  # if [[ -d "$ms_playwright_path" ]]; then
  #   local browser_count
  #   browser_count=$(find "$ms_playwright_path" -maxdepth 1 -type d | wc -l)
  #   if [[ "$browser_count" -gt 1 ]]; then
  #     log "Playwright browsers already installed at ${ms_playwright_path}, skipping installation"
  #     return 0
  #   fi
  # fi

  info "Installing Playwright browsers..."
  if [[ "$(uname -s)" == "Linux" ]]; then
    npm install @playwright/test
    npx playwright install-deps || exit 1
  else
    npm install @playwright/test
    npx playwright install || exit 1
  fi
}

# Handle playwright test result and exit appropriately
# Args: playwright_rc, test_description, rerun_command, [should_exit=true]
_handle_playwright_result() {
  local playwright_rc="$1"
  local test_description="$2"
  local rerun_command="$3"
  local should_exit="${4:-true}"

  if [[ $playwright_rc -eq 0 ]]; then
    log "✅  $test_description passed"
    if [[ "$should_exit" == "true" ]]; then
      exit 0
    fi
  else
    log "❌  $test_description failed with code $playwright_rc"
    echo ""
    echo "========================================"
    echo "To rerun this test locally, run:"
    echo "========================================"
    echo ""
    echo "  $rerun_command"
    echo ""
    echo "========================================"
    exit $playwright_rc
  fi
}

# Determine reporter based on show_html_report flag
# Args: current_reporter, show_html_report
_get_reporter() {
  local reporter="$1"
  local show_html_report="$2"

  if [[ "$show_html_report" == "true" ]]; then
    echo "html"
  else
    echo "$reporter"
  fi
}

# ==============================================================================
# Pod Health Check Functions (for spot instance resilience)
# ==============================================================================

# Check if all pods in namespace are Ready
# Returns 0 if all pods ready, 1 otherwise
# For pods to be considered ready:
#   - Completed/Succeeded pods (Jobs) are always considered ready
#   - Running pods must have all containers ready (e.g., 1/1, 2/2)
# Args: namespace, [kube_context]
_check_all_pods_ready() {
  local namespace="$1"
  local kube_context="${2:-}"
  local kubectl_cmd="kubectl"
  
  if [[ -n "$kube_context" ]]; then
    kubectl_cmd="kubectl --context=$kube_context"
  fi
  
  if [[ -z "$namespace" ]]; then
    log "WARNING: No namespace provided for pod check, skipping"
    return 0
  fi
  
  # Get pods that are NOT completed jobs AND are not fully ready
  # kubectl get pods output: NAME READY STATUS RESTARTS AGE
  # READY column (field 2) shows "X/Y" - we need X==Y for ready
  # STATUS column (field 3) shows Running, Completed, Succeeded, etc.
  local not_ready_pods
  not_ready_pods=$($kubectl_cmd get pods -n "$namespace" --no-headers 2>/dev/null | awk '
    # Skip completed jobs - they are always considered ready
    $3 == "Completed" || $3 == "Succeeded" { next }
    # For other pods, check if READY column shows all containers ready AND status is Running
    {
      split($2, ready, "/")
      if (ready[1] != ready[2] || $3 != "Running") {
        print $0
      }
    }
  ')
  
  if [[ -z "$not_ready_pods" ]]; then
    return 0
  else
    local count
    count=$(echo "$not_ready_pods" | wc -l | tr -d ' ')
    log "WARNING: $count pod(s) not ready in namespace $namespace:"
    while IFS= read -r line; do
      log "  $line"
    done <<< "$not_ready_pods"
    return 1
  fi
}

# Wait for all pods in namespace to be Ready
# Excludes completed Job pods (they use Succeeded status, not Ready condition)
# Args: namespace, [timeout_seconds=300], [kube_context]
_wait_for_pods_ready() {
  local namespace="$1"
  local timeout="${2:-300}"
  local kube_context="${3:-}"
  local kubectl_cmd="kubectl"
  
  if [[ -n "$kube_context" ]]; then
    kubectl_cmd="kubectl --context=$kube_context"
  fi
  
  if [[ -z "$namespace" ]]; then
    log "WARNING: No namespace provided for pod wait, skipping"
    return 0
  fi
  
  info "Waiting for pods to be ready in ${namespace} (timeout ${timeout}s)..."
  
  # Exclude completed Jobs (status.phase=Succeeded) - they don't have Ready condition
  if $kubectl_cmd wait --for=condition=Ready pods --all \
       --field-selector=status.phase!=Succeeded \
       -n "$namespace" \
       --timeout="${timeout}s" 2>/dev/null; then
    info "All pods ready in ${namespace}"
    return 0
  else
    # kubectl wait failed - this can happen if:
    # 1. Actual timeout (pods not ready)
    # 2. A pod was deleted during the wait (e.g., Error pod removed by controller)
    # 
    # Re-check if all current pods are now ready. If they are, the failed pod
    # was likely deleted and we can proceed.
    log "kubectl wait failed, verifying current pod state..."
    if _check_all_pods_ready "$namespace" "$kube_context"; then
      log "All current pods in namespace $namespace are Ready (previous failure may have been due to pod deletion)"
      return 0
    fi
    
    log "ERROR: Timeout waiting for pods to be Ready in namespace $namespace"
    _dump_pod_status "$namespace" "$kube_context"
    return 1
  fi
}

# Helper to dump pod status for diagnostics
# Args: namespace, [kube_context]
_dump_pod_status() {
  local namespace="$1"
  local kube_context="${2:-}"
  local kubectl_cmd="kubectl"

  if [[ -n "$kube_context" ]]; then
    kubectl_cmd="kubectl --context=$kube_context"
  fi

  log "Current pod status in namespace $namespace:"
  $kubectl_cmd get pods -n "$namespace" -o wide 2>/dev/null | while IFS= read -r line; do
    log "  $line"
  done
}

# Configuration for pod failure retry logic
_POD_RETRY_MAX_ATTEMPTS="${PLAYWRIGHT_POD_RETRY_MAX_ATTEMPTS:-2}"
_POD_RETRY_TIMEOUT="${PLAYWRIGHT_POD_RETRY_TIMEOUT:-420}"  # seconds

# Guard against invalid overrides.
if ! [[ "${_POD_RETRY_MAX_ATTEMPTS}" =~ ^[0-9]+$ ]]; then
  _POD_RETRY_MAX_ATTEMPTS=2
fi
if ! [[ "${_POD_RETRY_TIMEOUT}" =~ ^[0-9]+$ ]]; then
  _POD_RETRY_TIMEOUT=420
fi

# Run a playwright command with retry logic for pod failures (spot instance preemption)
# This function will retry the test if:
#   1. Pods are detected as not ready after test failure
#   2. Connection errors (ECONNREFUSED, etc.) are detected in test output
#      (handles race condition where pods recovered quickly after disruption)
# Args: namespace, kube_context, playwright_command...
# Returns: playwright exit code (0 = success, non-zero = failure)
_run_playwright_with_retry() {
  local namespace="$1"
  local kube_context="$2"
  shift 2
  local playwright_cmd=("$@")

  local attempt=0
  local playwright_rc=0
  local output_file=""
  
  # Cleanup temp file on function exit
  trap 'rm -f "$output_file"' RETURN
  
  while [[ $attempt -le $_POD_RETRY_MAX_ATTEMPTS ]]; do
    attempt=$((attempt + 1))
    
    # Check pods are ready before running
    if [[ -n "$namespace" ]]; then
      if ! _check_all_pods_ready "$namespace" "$kube_context"; then
        info "${COLOR_YELLOW}Pods not ready before attempt ${attempt}, waiting for recovery...${COLOR_RESET}"
        if ! _wait_for_pods_ready "$namespace" "$_POD_RETRY_TIMEOUT" "$kube_context"; then
          info "${COLOR_RED}Pods did not recover before attempt ${attempt}${COLOR_RESET}"
          if [[ $attempt -ge $_POD_RETRY_MAX_ATTEMPTS ]]; then
            info "${COLOR_RED}Max retry attempts reached, pods never recovered${COLOR_RESET}"
            return 1
          fi
          log "Will continue to next attempt..."
          continue
        fi
      fi
    fi
    
    if [[ $attempt -gt 1 ]]; then
      info "Retry attempt ${attempt}/${_POD_RETRY_MAX_ATTEMPTS} after pod recovery..."
    fi
    
    # Create temp file to capture output for connection error analysis
    # Clean up any previous iteration's file first
    rm -f "$output_file"
    output_file=$(mktemp)
    
    # Run the playwright command, capturing output for analysis.
    # IMPORTANT: We redirect to the file and separately stream to stdout
    # instead of piping through `tee`.  A pipeline (`cmd | tee file`)
    # keeps the pipe open until ALL writers close their inherited copy of
    # the write-end file descriptor.  If npx/node spawns child processes
    # (browser workers, etc.) that linger after the main process exits,
    # `tee` — and therefore the whole pipeline — hangs until those orphan
    # children terminate.  Using process substitution avoids this: the
    # shell only waits for the main command, not for the tee background
    # process.
    #
    # After the command exits we give the background `tee` a moment to
    # flush remaining output so the subsequent grep sees complete data.
    "${playwright_cmd[@]}" > >(tee "$output_file") 2>&1
    playwright_rc=$?
    sleep 1  # allow process-substitution tee to flush
    
    # If tests passed, we're done
    if [[ $playwright_rc -eq 0 ]]; then
      return 0
    fi
    
    # Tests failed - analyze why
    info "Tests failed (exit code ${playwright_rc}), checking infrastructure..."
    
    # Check for connection-related errors in output (infrastructure issues)
    local has_connection_error=false
    if grep -qiE "ECONNREFUSED|ECONNRESET|ETIMEDOUT|connection refused|connection reset|transport error|dial tcp.*connect:|Unavailable desc = connection error" "$output_file" 2>/dev/null; then
      has_connection_error=true
      info "${COLOR_YELLOW}Detected connection errors in test output (likely infrastructure issue)${COLOR_RESET}"
    fi
    
    if [[ -n "$namespace" ]]; then
      log "Checking pod health after test failure..."
      
      if ! _check_all_pods_ready "$namespace" "$kube_context"; then
        # Pods are not ready - this was likely a spot instance preemption
        info "${COLOR_YELLOW}Pods not ready after failure (possible spot preemption)${COLOR_RESET}"
        
        if [[ $attempt -lt $_POD_RETRY_MAX_ATTEMPTS ]]; then
          info "Waiting for pods to recover before retry..."
          if _wait_for_pods_ready "$namespace" "$_POD_RETRY_TIMEOUT" "$kube_context"; then
            info "Pods recovered, retrying tests..."
            continue
          else
            info "${COLOR_RED}Pods did not recover within timeout${COLOR_RESET}"
            return $playwright_rc
          fi
        else
          info "${COLOR_RED}Max retry attempts reached, pods still not ready${COLOR_RESET}"
          return $playwright_rc
        fi
      elif [[ "$has_connection_error" == "true" ]]; then
        # Pods are ready NOW, but we saw connection errors - likely recovered mid-test
        info "${COLOR_YELLOW}Pods are ready now but connection errors occurred during test${COLOR_RESET}"
        
        if [[ $attempt -lt $_POD_RETRY_MAX_ATTEMPTS ]]; then
          info "Pausing 10s for stability, then retrying..."
          sleep 10
          continue
        else
          info "${COLOR_RED}Max retry attempts reached${COLOR_RESET}"
          _dump_pod_status "$namespace" "$kube_context"
          return $playwright_rc
        fi
      else
        # Pods are ready and no connection errors - legitimate test failure
        log "Pods are healthy and no connection errors detected - this appears to be a legitimate test failure"
        _dump_pod_status "$namespace" "$kube_context"
        return $playwright_rc
      fi
    else
      # No namespace provided, can't check pods - return the failure
      return $playwright_rc
    fi
  done
  
  return $playwright_rc
}

# ==============================================================================
# Main Playwright Test Functions
# ==============================================================================

run_playwright_tests() {
  local test_suite_path="$1"
  local show_html_report="$2"
  local shard_index="$3"
  local shard_total="$4"
  local reporter="$5"
  local test_exclude="$6"
  local run_smoke_tests="$7"
  local enable_debug="$8"
  local namespace="${9:-}"  # Optional: namespace for pod health checks
  local kube_context="${10:-}"  # Optional: kubernetes context
  local rerun_cmd="${11:-}"  # Optional: command to rerun tests locally

  log "Smoke tests: $run_smoke_tests"
  log "Reporter: $reporter"
  [[ -n "$namespace" ]] && log "Namespace for pod checks: $namespace"
  [[ -n "$kube_context" ]] && log "Kube context: $kube_context"

  _setup_playwright_environment "$test_suite_path" "false"
  _install_playwright_browsers

  _log_e2e_suite_version

  reporter=$(_get_reporter "$reporter" "$show_html_report")

  # Enable Playwright debug and traces if requested
  local trace_flag=""
  if [[ "$enable_debug" == "true" ]]; then
    export DEBUG="${DEBUG:-pw:api,pw:browser*}"
    if [[ -z "${PLAYWRIGHT_E2E_TRACE:-}" ]]; then
      trace_flag="--trace=retain-on-failure"
    fi
    log "Playwright DEBUG enabled: $DEBUG"
  fi

  local project="full-suite"
  if [[ "$run_smoke_tests" == "true" ]]; then
    project="smoke-tests"
    info "Running smoke tests..."
  else
    info "Running Playwright tests..."
  fi

  # Build the playwright command arguments
  local -a playwright_args=(
    npx playwright test
    --project="$project"
    --shard="${shard_index}/${shard_total}"
    --reporter="$reporter,json"
  )
  [[ -n "$test_exclude" ]] && playwright_args+=(--grep-invert="$test_exclude")
  [[ -n "$trace_flag" ]] && playwright_args+=($trace_flag)
  [[ -n "${PLAYWRIGHT_E2E_VIDEO:-}" ]] && playwright_args+=(--video="$PLAYWRIGHT_E2E_VIDEO")
  [[ -n "${PLAYWRIGHT_E2E_TRACE:-}" ]] && playwright_args+=(--trace="$PLAYWRIGHT_E2E_TRACE")
  [[ -n "${PLAYWRIGHT_E2E_RETRIES:-}" ]] && playwright_args+=(--retries="$PLAYWRIGHT_E2E_RETRIES")
  # Namespace-scoped output directory to avoid collisions during parallel matrix runs
  [[ -n "${PLAYWRIGHT_TEST_OUTPUT:-}" ]] && playwright_args+=(--output="$PLAYWRIGHT_TEST_OUTPUT")

  # Run with retry logic for pod failures (spot instance preemption)
  PLAYWRIGHT_JSON_OUTPUT_NAME=test-results/playwright-results.json \
    _run_playwright_with_retry "$namespace" "$kube_context" "${playwright_args[@]}"
  local playwright_rc=$?

  # Only show HTML report locally, never in CI (it blocks waiting for Ctrl+C)
  if [[ "$show_html_report" == "true" && "${CI:-false}" != "true" ]]; then
    npx playwright show-report "${PLAYWRIGHT_HTML_REPORT:-playwright-report}"
  fi

  _handle_playwright_result "$playwright_rc" "All Playwright tests" "$rerun_cmd" "true"
}

# Run playwright tests for hybrid auth - runs specific test files with a specific auth type
# This function does NOT exit on success so multiple phases can run sequentially
run_playwright_tests_hybrid() {
  local test_suite_path="$1"
  local show_html_report="$2"
  local auth_type="$3"
  local test_files="$4"
  local test_exclude="$5"
  local namespace="${6:-}"  # Optional: namespace for pod health checks
  local kube_context="${7:-}"  # Optional: kubernetes context
  local rerun_cmd="${8:-}"  # Optional: command to rerun tests locally

  info "Running hybrid tests (auth=${auth_type}): ${test_files}"
  [[ -n "$namespace" ]] && log "Namespace for pod checks: $namespace"
  [[ -n "$kube_context" ]] && log "Kube context: $kube_context"

  _setup_playwright_environment "$test_suite_path" "true"

  _log_e2e_suite_version

  local reporter
  reporter=$(_get_reporter "html" "$show_html_report")

  # Build the playwright command arguments
  # shellcheck disable=SC2206
  local -a playwright_args=(npx playwright test $test_files --project=full-suite --reporter="$reporter,json")
  [[ -n "$test_exclude" ]] && playwright_args+=(--grep-invert="$test_exclude")
  # Namespace-scoped output directory to avoid collisions during parallel matrix runs
  [[ -n "${PLAYWRIGHT_TEST_OUTPUT:-}" ]] && playwright_args+=(--output="$PLAYWRIGHT_TEST_OUTPUT")

  # Run specific test files with the auth type set as environment variable
  # This overrides any TEST_AUTH_TYPE in .env file
  # Run with retry logic for pod failures (spot instance preemption)
  PLAYWRIGHT_JSON_OUTPUT_NAME=test-results/playwright-results.json \
    TEST_AUTH_TYPE="$auth_type" _run_playwright_with_retry "$namespace" "$kube_context" "${playwright_args[@]}"
  local playwright_rc=$?

  _handle_playwright_result "$playwright_rc" "Hybrid Playwright tests ($auth_type)" "$rerun_cmd" "false"
}
