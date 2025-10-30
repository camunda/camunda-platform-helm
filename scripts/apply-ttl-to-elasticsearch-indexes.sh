#!/usr/bin/env bash
#
# cleanup-indexes.sh — Delete Elasticsearch indexes older than a TTL for a given prefix.
#
# Usage:
#   ./cleanup-indexes.sh --prefix logs- --ttl 2h [--url http://localhost:9200] [--user elastic] [--pass changeme] [--dry-run]
#
# TTL supports s/m/h/d suffixes (e.g., 45m, 2h, 1d).
#

set -euo pipefail

log() { echo "[$(date -u '+%Y-%m-%dT%H:%M:%SZ')] $*"; }
debug() { [[ "${DEBUG:-0}" == "1" ]] && echo "[DEBUG] $*"; }

# Error handler and usage
on_error() {
  local line=$1
  log "❌ Error on or near line ${line}. Enable --debug for more details."
}
trap 'on_error $LINENO' ERR

usage() {
  cat <<EOF
Usage: $0 --prefix <prefix> --ttl <1h|30m|1d> [--url <url>] [--user <user>] [--pass <pass>] [--dry-run] [--debug]

Options:
  --prefix    Index prefix to match (e.g., logs-)
  --ttl       Time-to-live threshold; delete indices older than this (s/m/h/d)
  --url       Elasticsearch base URL (default: http://localhost:9200)
  --user      Username for basic auth
  --pass      Password for basic auth
  --dry-run   Show what would be deleted, but do not delete
  --debug     Verbose debug logging
  -h, --help  Show this help and exit
EOF
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || { log "❌ Required command not found: $1"; exit 127; }
}

# Curl wrapper capturing status and body
REQ_STATUS=""
REQ_BODY=""
curl_request() {
  local method=$1
  local url=$2
  shift 2
  local tmp
  tmp=$(mktemp)
  local code
  code=$(curl --silent --show-error --connect-timeout 5 --max-time 30 \
    --retry 2 --retry-delay 1 --retry-connrefused \
    -o "$tmp" -w "%{http_code}" -X "$method" "${auth_args[@]}" "$url" "$@") || true
  if [[ "$code" =~ ^[0-9]{3}$ ]]; then
    REQ_STATUS="$code"
  else
    REQ_STATUS="000" # network/transport error
  fi
  REQ_BODY="$(cat "$tmp")"
  rm -f "$tmp"
  debug "HTTP ${method} ${url} -> ${REQ_STATUS}"
  return 0
}

PREFIX=""
TTL=""
ES_URL="http://localhost:9200"
ES_USER=""
ES_PASS=""
DRY_RUN=0

# ---- argument parsing ----
while [[ $# -gt 0 ]]; do
  case "$1" in
    --prefix)
      PREFIX="$2"
      shift 2
      ;;
    --ttl)
      TTL="$2"
      shift 2
      ;;
    --url)
      ES_URL="$2"
      shift 2
      ;;
    --user)
      ES_USER="$2"
      shift 2
      ;;
    --pass)
      ES_PASS="$2"
      shift 2
      ;;
    --dry-run)
      DRY_RUN=1
      shift
      ;;
    --debug)
      DEBUG=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      log "❌ Unknown arg: $1"
      usage
      exit 1
      ;;
  esac
done

if [[ -z "$PREFIX" || -z "$TTL" ]]; then
  usage
  exit 1
fi

# Validate dependencies
require_cmd curl
require_cmd jq

# ---- helpers ----
auth_args=()
[[ -n "$ES_USER" && -n "$ES_PASS" ]] && auth_args=(-u "${ES_USER}:${ES_PASS}")

ttl_to_seconds() {
  case "$1" in
    *d) echo $((${1%d} * 86400)) ;;
    *h) echo $((${1%h} * 3600)) ;;
    *m) echo $((${1%m} * 60)) ;;
    *s) echo $((${1%s})) ;;
    *) echo "$1" ;;
  esac
}

if ! [[ "$TTL" =~ ^[0-9]+([smhd])$ ]]; then
  log "❌ Invalid TTL: '$TTL'. Expected format like 45m, 2h, 1d, 30s"
  exit 1
fi

TTL_SECS=$(ttl_to_seconds "$TTL")
NOW_MS=$(($(date +%s) * 1000))

# ---- main ----
log "🔍 Checking indexes matching '${PREFIX}*' older than ${TTL}"
curl_request GET "${ES_URL}/_cat/indices/${PREFIX}*?h=index"
if ! [[ "$REQ_STATUS" =~ ^2..$ ]]; then
  log "❌ Failed to list indices (HTTP $REQ_STATUS)"
  [[ -n "$REQ_BODY" ]] && echo "$REQ_BODY" | sed 's/^/  > /'
  exit 1
fi
INDICES="$REQ_BODY"

if [[ -z "$INDICES" ]]; then
  log "✅ No indexes found for prefix '${PREFIX}'"
  exit 0
fi

declare -a EXPIRED=()
declare -a DELETED=()
declare -a FAILED=()
declare -a FOUND=()
declare -a NOT_FOUND=()

while read -r INDEX; do
  [[ -z "$INDEX" ]] && continue
  curl_request GET "${ES_URL}/${INDEX}/_settings"
  if ! [[ "$REQ_STATUS" =~ ^2..$ ]]; then
    log "⚠️  Failed to get settings for ${INDEX} (HTTP $REQ_STATUS) — skipping"
    continue
  fi
  CREATED=$(echo "$REQ_BODY" | jq -r ".[\"${INDEX}\"].settings.index.creation_date" 2>/dev/null || echo "null")
  if [[ "$CREATED" == "null" || -z "$CREATED" || ! "$CREATED" =~ ^[0-9]+$ ]]; then
    log "⚠️  Could not determine creation_date for ${INDEX} — skipping"
    continue
  fi
  AGE=$(((NOW_MS - CREATED) / 1000))
  if ((AGE > TTL_SECS)); then
    EXPIRED+=("$INDEX")
  fi
done <<< "$INDICES"

if ((${#EXPIRED[@]} == 0)); then
  log "✅ No expired indexes found."
  exit 0
fi

log "🗑 Found ${#EXPIRED[@]} expired indexes:"
for IDX in "${EXPIRED[@]}"; do echo "  - $IDX"; done

if ((DRY_RUN == 1)); then
  log "🔎 Verifying existence of expired indexes (dry run)..."
  for IDX in "${EXPIRED[@]}"; do
    curl_request HEAD "${ES_URL}/${IDX}"
    case "$REQ_STATUS" in
      200|204)
        log "✅ ${IDX} exists"
        FOUND+=("$IDX")
        ;;
      404)
        log "ℹ️  ${IDX} not found"
        NOT_FOUND+=("$IDX")
        ;;
      *)
        log "⚠️  ${IDX} existence check failed (HTTP $REQ_STATUS)"
        ;;
    esac
  done
  log "🧪 Dry run only — no deletions performed."
  log "📊 Summary: matched=${#EXPIRED[@]} found=${#FOUND[@]} not_found=${#NOT_FOUND[@]}"
  exit 0
fi

for IDX in "${EXPIRED[@]}"; do
  # Check existence before trying to delete to report clearly
  curl_request HEAD "${ES_URL}/${IDX}"
  case "$REQ_STATUS" in
    200|204)
      log "🔎 ${IDX} exists"
      FOUND+=("$IDX")
      ;;
    404)
      log "ℹ️  ${IDX} not found — skipping delete"
      NOT_FOUND+=("$IDX")
      continue
      ;;
    *)
      log "⚠️  ${IDX} existence check failed (HTTP $REQ_STATUS) — attempting delete anyway"
      ;;
  esac
  log "Deleting ${IDX} ..."
  curl_request DELETE "${ES_URL}/${IDX}"
  case "$REQ_STATUS" in
    200|202)
      log "✅ Deleted ${IDX}"
      DELETED+=("$IDX")
      ;;
    404)
      log "ℹ️  Index ${IDX} not found (404) — skipping"
      ;;
    *)
      log "❌ Failed to delete ${IDX} (HTTP $REQ_STATUS)"
      [[ -n "$REQ_BODY" ]] && echo "$REQ_BODY" | sed 's/^/  > /'
      FAILED+=("$IDX")
      ;;
  esac
done

log "🏁 Cleanup complete."
log "📊 Summary: matched=${#EXPIRED[@]} found=${#FOUND[@]} not_found=${#NOT_FOUND[@]} deleted=${#DELETED[@]} failed=${#FAILED[@]}"
if ((${#FAILED[@]} > 0)); then
  log "❗ Failures:"
  for f in "${FAILED[@]}"; do echo "  - $f"; done
  exit 2
fi
