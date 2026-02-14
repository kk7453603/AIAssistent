#!/usr/bin/env bash
set -euo pipefail

MANIFEST=""
OUT="./tmp/eval/retrieval_cases.jsonl"

usage() {
  cat <<'EOF'
Generate retrieval evaluation cases from a synthetic manifest.csv.

Usage:
  scripts/eval/generate_cases_from_manifest.sh --manifest ./tmp/rag-300/manifest.csv [--out ./tmp/eval/retrieval_cases.jsonl]
EOF
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Required command is missing: $1" >&2
    exit 1
  fi
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --manifest)
      MANIFEST="${2:-}"
      shift 2
      ;;
    --out)
      OUT="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage
      exit 1
      ;;
  esac
done

if [[ -z "$MANIFEST" ]]; then
  echo "--manifest is required" >&2
  usage
  exit 1
fi

if [[ ! -f "$MANIFEST" ]]; then
  echo "manifest file not found: $MANIFEST" >&2
  exit 1
fi

require_cmd jq
mkdir -p "$(dirname "$OUT")"
: >"$OUT"

index=0
while IFS=$'\t' read -r filename region product quarter; do
  index=$((index + 1))
  case_id="$(printf "RET%04d" "$index")"

  jq -nc \
    --arg id "$case_id" \
    --arg question "What is the risk level for document ${filename}?" \
    --arg filename "$filename" \
    '{id:$id,question:$question,expected_filenames:[$filename]}' >>"$OUT"

  index=$((index + 1))
  case_id="$(printf "RET%04d" "$index")"

  jq -nc \
    --arg id "$case_id" \
    --arg question "What is the gross margin for document ${filename} in ${quarter}?" \
    --arg filename "$filename" \
    '{id:$id,question:$question,expected_filenames:[$filename]}' >>"$OUT"
done < <(awk -F',' 'NR > 1 {print $1 "\t" $4 "\t" $5 "\t" $12}' "$MANIFEST")

echo "Generated $(wc -l < "$OUT") retrieval eval cases: $OUT"
