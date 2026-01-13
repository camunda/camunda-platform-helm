#!/usr/bin/env bash
set -euox pipefail

if ! command -v go > /dev/null 2>&1; then
  echo "[ERR ] Go toolchain not found on PATH; cannot run prepare-helm-values CLI" >&2
  exit 127
fi

# Execute the Go CLI within its module directory so Go picks up the nested go.mod
script_dir="$(cd -- "$(dirname "$0")" > /dev/null 2>&1 && pwd)"
cli_dir="${script_dir}/prepare-helm-values"

cd "$cli_dir"
# Ensure dependencies and go.sum are present
go mod tidy -v
exec go run . "$@"
