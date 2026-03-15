#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: scripts/obsidian/sync_vault.sh --vault /path/to/vault [options]

Options:
  --api-url URL         API base URL (default: http://localhost:8080)
  --state PATH          State file path (default: ./tmp/obsidian-sync/state.tsv)
  --wait-ready          Poll /v1/documents/{id} until ready/failed
  --poll-interval SEC   Poll interval (default: 2)
  --poll-timeout SEC    Poll timeout per document (default: 600)
  -h, --help            Show help
EOF
}

API_URL="http://localhost:8080"
STATE_PATH="./tmp/obsidian-sync/state.tsv"
WAIT_READY="false"
POLL_INTERVAL="2"
POLL_TIMEOUT="600"
VAULT_PATH=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --vault)
      VAULT_PATH="${2:-}"
      shift 2
      ;;
    --api-url)
      API_URL="${2:-}"
      shift 2
      ;;
    --state)
      STATE_PATH="${2:-}"
      shift 2
      ;;
    --wait-ready)
      WAIT_READY="true"
      shift 1
      ;;
    --poll-interval)
      POLL_INTERVAL="${2:-}"
      shift 2
      ;;
    --poll-timeout)
      POLL_TIMEOUT="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [[ -z "$VAULT_PATH" ]]; then
  echo "--vault is required" >&2
  usage >&2
  exit 1
fi

if [[ ! -d "$VAULT_PATH" ]]; then
  echo "Vault not found: $VAULT_PATH" >&2
  exit 1
fi

if ! command -v curl >/dev/null 2>&1; then
  echo "curl not found" >&2
  exit 1
fi

if ! command -v sha256sum >/dev/null 2>&1; then
  echo "sha256sum not found" >&2
  exit 1
fi

mkdir -p "$(dirname "$STATE_PATH")"
LOCK_PATH="${STATE_PATH}.lock"
if ! mkdir "$LOCK_PATH" 2>/dev/null; then
  echo "Sync lock is held, skipping: $LOCK_PATH" >&2
  exit 0
fi
trap 'rmdir "$LOCK_PATH"' EXIT

declare -A known_hash
declare -A known_doc

if [[ -f "$STATE_PATH" ]]; then
  while IFS=$'\t' read -r path hash doc_id; do
    [[ -n "$path" ]] || continue
    known_hash["$path"]="$hash"
    known_doc["$path"]="$doc_id"
  done <"$STATE_PATH"
fi

state_tmp="$(mktemp)"
uploaded=0
skipped=0
failed=0

upload_file() {
  local file="$1"
  local response http_code doc_id status
  response="$(mktemp)"

  http_code="$(curl -sS -o "$response" -w "%{http_code}" -X POST \
    -F "file=@${file}" \
    "${API_URL}/v1/documents")"

  if [[ "$http_code" != "202" ]]; then
    echo "Upload failed (${http_code}): ${file}" >&2
    cat "$response" >&2 || true
    rm -f "$response"
    return 1
  fi

  doc_id="$(sed -n 's/.*\"id\":\"\\([^\"]*\\)\".*/\\1/p' "$response" | head -n 1)"
  rm -f "$response"
  if [[ -z "$doc_id" ]]; then
    echo "Failed to extract document_id from response for ${file}" >&2
    return 1
  fi

  if [[ "$WAIT_READY" == "true" ]]; then
    local started
    started="$(date +%s)"
    while true; do
      status="$(curl -sS "${API_URL}/v1/documents/${doc_id}" | sed -n 's/.*\"status\":\"\\([^\"]*\\)\".*/\\1/p' | head -n 1)"
      if [[ "$status" == "ready" ]]; then
        break
      fi
      if [[ "$status" == "failed" ]]; then
        echo "Document processing failed: ${file} (${doc_id})" >&2
        return 1
      fi
      if [[ -n "$status" ]]; then
        :
      fi
      if (( $(date +%s) - started > POLL_TIMEOUT )); then
        echo "Timeout waiting for ready: ${file} (${doc_id})" >&2
        return 1
      fi
      sleep "$POLL_INTERVAL"
    done
  fi

  echo "$doc_id"
  return 0
}

while IFS= read -r -d '' file; do
  rel_path="${file#"$VAULT_PATH"/}"
  hash="$(sha256sum "$file" | awk '{print $1}')"

  if [[ "${known_hash[$rel_path]:-}" == "$hash" ]]; then
    echo -e "${rel_path}\t${hash}\t${known_doc[$rel_path]:-}" >>"$state_tmp"
    skipped=$((skipped + 1))
    continue
  fi

  if doc_id="$(upload_file "$file")"; then
    echo -e "${rel_path}\t${hash}\t${doc_id}" >>"$state_tmp"
    uploaded=$((uploaded + 1))
  else
    prev_hash="${known_hash[$rel_path]:-}"
    prev_doc="${known_doc[$rel_path]:-}"
    echo -e "${rel_path}\t${prev_hash}\t${prev_doc}" >>"$state_tmp"
    failed=$((failed + 1))
  fi
done < <(find "$VAULT_PATH" -type f -name '*.md' \
  ! -path '*/.obsidian/*' ! -path '*/.trash/*' ! -path '*/.git/*' -print0)

mv "$state_tmp" "$STATE_PATH"

echo "Done. Uploaded: ${uploaded}, skipped: ${skipped}, errors: ${failed}"
