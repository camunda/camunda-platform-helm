#!/usr/bin/env bats

# Tests for scripts/check-no-plaintext-datastore.sh.
# Stubs `kubectl` to return canned pod JSON so we can exercise the regex
# matcher against representative TLS / non-TLS / mixed-mode env vars.

setup() {
  here="$(cd "$(dirname "${BATS_TEST_FILENAME}")" && pwd)"
  if ROOT="$(git -C "$here" rev-parse --show-toplevel 2>/dev/null)"; then
    :
  else
    ROOT="$(cd "$here/../.." && pwd)"
  fi
  export ROOT
  export SCRIPT="$ROOT/scripts/check-no-plaintext-datastore.sh"

  TMPDIR_TEST="$(mktemp -d)"
  export TMPDIR_TEST
  export PATH="$TMPDIR_TEST/bin:$PATH"
  mkdir -p "$TMPDIR_TEST/bin"
}

teardown() {
  rm -rf "$TMPDIR_TEST"
}

# Helper: install a kubectl shim that returns canned pod JSON for
# `kubectl ... get pod <name> -o json`, and pod names for the
# `get pods -o jsonpath` invocation.
install_kubectl_stub() {
  local pods="$1"
  local pod_json_dir="$2"
  cat > "$TMPDIR_TEST/bin/kubectl" <<'STUBEOF'
#!/usr/bin/env bash
args=("$@")
case " ${args[*]} " in
  *" get pods "*"jsonpath="*)
    echo "PODS_PLACEHOLDER"
    ;;
  *" get pod "*)
    for ((i=0; i < ${#args[@]}; i++)); do
      if [[ "${args[$i]}" == "pod" ]]; then
        cat "JSON_DIR_PLACEHOLDER/${args[$((i+1))]}.json" 2>/dev/null
        exit $?
      fi
    done
    echo "stub: no pod name found" >&2; exit 2
    ;;
  *)
    echo "stub: unsupported invocation: ${args[*]}" >&2; exit 2
    ;;
esac
STUBEOF
  sed -i "s|PODS_PLACEHOLDER|${pods}|" "$TMPDIR_TEST/bin/kubectl"
  sed -i "s|JSON_DIR_PLACEHOLDER|${pod_json_dir}|" "$TMPDIR_TEST/bin/kubectl"
  chmod +x "$TMPDIR_TEST/bin/kubectl"
}

# Helper: write a pod JSON file with env vars for one container.
# Arguments: <dir> <pod> <container> [env-name env-value]...
write_pod_json() {
  local dir="$1" pod="$2" container="$3"
  shift 3
  mkdir -p "$dir"
  local envs="["
  local first=1
  while (( $# > 1 )); do
    if (( first )); then first=0; else envs+=","; fi
    envs+="{\"name\": \"$1\", \"value\": \"$2\"}"
    shift 2
  done
  envs+="]"
  cat > "$dir/$pod.json" <<EOF
{
  "metadata": {"name": "$pod"},
  "spec": {
    "containers": [{"name": "$container", "env": $envs}],
    "initContainers": []
  }
}
EOF
}

@test "PASS: TLS-only deployment with HTTPS + JDBC sslmode=verify-full" {
  write_pod_json "$TMPDIR_TEST/pods" "orchestration-0" "orchestration" \
    "OS_URL"   "https://opensearch-master:9200" \
    "JDBC_URL" "jdbc:postgresql://postgres-tls-postgresql:5432/orchestration?sslmode=verify-full&sslrootcert=/etc/camunda/tls/ca.crt"
  install_kubectl_stub "orchestration-0" "$TMPDIR_TEST/pods"

  run bash "$SCRIPT" --namespace ci-test
  [ "$status" -eq 0 ]
  [[ "$output" == *"PASS"* ]]
}

@test "FAIL: plaintext HTTP to opensearch-master" {
  write_pod_json "$TMPDIR_TEST/pods" "orchestration-0" "orchestration" \
    "OS_URL" "http://opensearch-master:9200"
  install_kubectl_stub "orchestration-0" "$TMPDIR_TEST/pods"

  run bash "$SCRIPT" --namespace ci-test
  [ "$status" -eq 1 ]
  [[ "$output" == *"PLAINTEXT-HTTP"* ]]
}

@test "FAIL: plaintext HTTP to integration-elasticsearch" {
  write_pod_json "$TMPDIR_TEST/pods" "orchestration-0" "orchestration" \
    "ES_URL" "http://integration-elasticsearch:9200"
  install_kubectl_stub "orchestration-0" "$TMPDIR_TEST/pods"

  run bash "$SCRIPT" --namespace ci-test
  [ "$status" -eq 1 ]
}

@test "FAIL: JDBC URL missing sslmode" {
  write_pod_json "$TMPDIR_TEST/pods" "orchestration-0" "orchestration" \
    "JDBC_URL" "jdbc:postgresql://postgres-tls-postgresql:5432/orchestration"
  install_kubectl_stub "orchestration-0" "$TMPDIR_TEST/pods"

  run bash "$SCRIPT" --namespace ci-test
  [ "$status" -eq 1 ]
  [[ "$output" == *"INSECURE-JDBC"* ]]
}

@test "FAIL: JDBC URL with ssl=true alone (no sslmode=verify-*)" {
  write_pod_json "$TMPDIR_TEST/pods" "orchestration-0" "orchestration" \
    "JDBC_URL" "jdbc:postgresql://postgres-tls-postgresql:5432/orchestration?ssl=true"
  install_kubectl_stub "orchestration-0" "$TMPDIR_TEST/pods"

  run bash "$SCRIPT" --namespace ci-test
  [ "$status" -eq 1 ]
}

@test "PASS: unrelated HTTP URL (actuator endpoint) does not match" {
  write_pod_json "$TMPDIR_TEST/pods" "orchestration-0" "orchestration" \
    "ACTUATOR" "http://localhost:9600/actuator"
  install_kubectl_stub "orchestration-0" "$TMPDIR_TEST/pods"

  run bash "$SCRIPT" --namespace ci-test
  [ "$status" -eq 0 ]
}

@test "ERROR: missing --namespace argument" {
  run bash "$SCRIPT"
  [ "$status" -eq 2 ]
}

@test "ERROR: --namespace with no value exits 2 (not 1 from set -u)" {
  run bash "$SCRIPT" --namespace
  [ "$status" -eq 2 ]
  [[ "$output" == *"requires a value"* ]]
}

@test "ERROR: --kube-context with no value exits 2 (not 1 from set -u)" {
  run bash "$SCRIPT" --namespace ci-test --kube-context
  [ "$status" -eq 2 ]
}

@test "FAIL: plaintext HTTP to elasticsearch-master-headless suffix" {
  write_pod_json "$TMPDIR_TEST/pods" "orchestration-0" "orchestration" \
    "ES_URL" "http://elasticsearch-master-headless:9200"
  install_kubectl_stub "orchestration-0" "$TMPDIR_TEST/pods"

  run bash "$SCRIPT" --namespace ci-test
  [ "$status" -eq 1 ]
}

@test "FAIL: plaintext HTTP to opensearch-master-coordinating-only suffix" {
  write_pod_json "$TMPDIR_TEST/pods" "orchestration-0" "orchestration" \
    "OS_URL" "http://opensearch-master-coordinating-only:9200"
  install_kubectl_stub "orchestration-0" "$TMPDIR_TEST/pods"

  run bash "$SCRIPT" --namespace ci-test
  [ "$status" -eq 1 ]
}

@test "FAIL: multiple JDBC URLs in one env var, second is insecure" {
  write_pod_json "$TMPDIR_TEST/pods" "orchestration-0" "orchestration" \
    "JDBC_PAIR" "jdbc:postgresql://secure:5432/db?sslmode=verify-full&jdbc:postgresql://insecure:5432/db"
  install_kubectl_stub "orchestration-0" "$TMPDIR_TEST/pods"

  run bash "$SCRIPT" --namespace ci-test
  [ "$status" -eq 1 ]
  [[ "$output" == *"insecure"* ]]
}

@test "FAIL: JDBC URL with sslmode=require (disables hostname verification)" {
  write_pod_json "$TMPDIR_TEST/pods" "orchestration-0" "orchestration" \
    "JDBC_URL" "jdbc:postgresql://postgres-tls-postgresql:5432/orchestration?sslmode=require"
  install_kubectl_stub "orchestration-0" "$TMPDIR_TEST/pods"

  run bash "$SCRIPT" --namespace ci-test
  [ "$status" -eq 1 ]
  [[ "$output" == *"INSECURE-JDBC"* ]]
}

@test "FAIL: plaintext HTTP in initContainer env var" {
  # Write pod JSON with a clean main container and a violating initContainer.
  mkdir -p "$TMPDIR_TEST/pods"
  cat > "$TMPDIR_TEST/pods/orchestration-0.json" <<EOF
{
  "metadata": {"name": "orchestration-0"},
  "spec": {
    "containers": [{"name": "orchestration", "env": [{"name": "OS_URL", "value": "https://opensearch-master:9200"}]}],
    "initContainers": [{"name": "init-tls", "env": [{"name": "OS_URL", "value": "http://opensearch-master:9200"}]}]
  }
}
EOF
  install_kubectl_stub "orchestration-0" "$TMPDIR_TEST/pods"

  run bash "$SCRIPT" --namespace ci-test
  [ "$status" -eq 1 ]
  [[ "$output" == *"PLAINTEXT-HTTP"* ]]
  [[ "$output" == *"init-tls"* ]]
}
