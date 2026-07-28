#!/usr/bin/env bash
set -euo pipefail

cd "$(git rev-parse --show-toplevel)/scripts/capacity-manager"
go run ./cmd/verify
