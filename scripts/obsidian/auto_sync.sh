#!/usr/bin/env bash
set -euo pipefail

CONFIG_PATH="${ASSISTANT_OBSIDIAN_CONFIG_PATH:-/app/backend/data/assistant/obsidian_vaults.json}"
STATE_DIR="${ASSISTANT_OBSIDIAN_STATE_DIR:-/app/backend/data/assistant/obsidian_state}"
API_URL="${ASSISTANT_API_URL:-http://api:8080}"
DEFAULT_INTERVAL_MINUTES="${ASSISTANT_OBSIDIAN_DEFAULT_INTERVAL_MINUTES:-15}"
POLL_SECONDS="${ASSISTANT_OBSIDIAN_POLL_SECONDS:-30}"
WAIT_READY="${ASSISTANT_OBSIDIAN_WAIT_READY:-true}"
SYNC_SCRIPT="${ASSISTANT_OBSIDIAN_SYNC_SCRIPT:-/sync/sync_vault.sh}"

mkdir -p "$STATE_DIR"

log() {
  printf '[obsidian-sync] %s\n' "$*"
}

read_config() {
  if [[ ! -f "$CONFIG_PATH" ]]; then
    echo ""
    return 0
  fi
  jq -c '.vaults[]? // empty' "$CONFIG_PATH" 2>/dev/null || true
}

meta_path() {
  local vault_id="$1"
  echo "$STATE_DIR/${vault_id}.meta.json"
}

load_last_sync() {
  local meta="$1"
  if [[ -f "$meta" ]]; then
    jq -r '.last_sync_epoch // 0' "$meta" 2>/dev/null || echo "0"
  else
    echo "0"
  fi
}

write_meta() {
  local meta="$1"
  local status="$2"
  local err="$3"
  local now
  now="$(date +%s)"
  jq -n \
    --arg status "$status" \
    --arg error "$err" \
    --argjson ts "$now" \
    '{last_status: $status, last_error: $error, last_sync_epoch: $ts}' >"$meta"
}

sync_vault() {
  local name="$1"
  local vault_id="$2"
  local path="$3"
  local interval="$4"
  local enabled="$5"

  if [[ "$enabled" != "true" ]]; then
    return 0
  fi

  if [[ ! -d "$path" ]]; then
    write_meta "$(meta_path "$vault_id")" "error" "vault path not found: $path"
    log "vault '$name' path not found: $path"
    return 0
  fi

  local meta
  meta="$(meta_path "$vault_id")"
  local last
  last="$(load_last_sync "$meta")"
  local now
  now="$(date +%s)"
  if ! [[ "$interval" =~ ^[0-9]+$ ]]; then
    interval="$DEFAULT_INTERVAL_MINUTES"
  fi
  local interval_sec=$((interval * 60))
  if (( interval_sec <= 0 )); then
    interval_sec=$((DEFAULT_INTERVAL_MINUTES * 60))
  fi
  if (( now - last < interval_sec )); then
    return 0
  fi

  local state_file="$STATE_DIR/${vault_id}.tsv"
  local args=("--vault" "$path" "--api-url" "$API_URL" "--state" "$state_file")
  if [[ "$WAIT_READY" == "true" ]]; then
    args+=("--wait-ready")
  fi

  log "syncing '$name' from $path"
  if "$SYNC_SCRIPT" "${args[@]}"; then
    write_meta "$meta" "ok" ""
    log "sync ok '$name'"
  else
    write_meta "$meta" "error" "sync_vault failed"
    log "sync failed '$name'"
  fi
}

if ! command -v jq >/dev/null 2>&1; then
  log "jq is required"
  exit 1
fi

if [[ ! -x "$SYNC_SCRIPT" ]]; then
  log "sync script not found or not executable: $SYNC_SCRIPT"
  exit 1
fi

log "started (config=$CONFIG_PATH, state=$STATE_DIR)"

while true; do
  any=0
  while IFS= read -r vault_json; do
    [[ -n "$vault_json" ]] || continue
    any=1
    name="$(echo "$vault_json" | jq -r '.name // empty')"
    vault_id="$(echo "$vault_json" | jq -r '.id // empty')"
    path="$(echo "$vault_json" | jq -r '.path // empty')"
    enabled="$(echo "$vault_json" | jq -r '.enabled // true')"
    interval="$(echo "$vault_json" | jq -r '.interval_minutes // empty')"
    if [[ -z "$name" || -z "$path" ]]; then
      continue
    fi
    if [[ -z "$vault_id" || "$vault_id" == "null" ]]; then
      vault_id="$(echo "$name" | tr '[:upper:]' '[:lower:]' | tr -c 'a-z0-9_.-' '_' )"
    fi
    if [[ -z "$interval" || "$interval" == "null" ]]; then
      interval="$DEFAULT_INTERVAL_MINUTES"
    fi
    sync_vault "$name" "$vault_id" "$path" "$interval" "$enabled"
  done < <(read_config)

  if [[ "$any" -eq 0 ]]; then
    log "no vaults configured"
  fi
  sleep "$POLL_SECONDS"
done
