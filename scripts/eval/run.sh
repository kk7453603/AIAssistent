#!/usr/bin/env bash
set -euo pipefail

API_URL="http://localhost:8080"
CASES_FILE="scripts/eval/cases.example.jsonl"
TOP_K=5
OUT="./tmp/eval/report.json"

usage() {
  cat <<'EOF'
Run retrieval evaluation against /v1/rag/query.

Case format: JSONL, one object per line
  {"id":"Q1","question":"...","expected_filenames":["doc_0001.txt"]}

Usage:
  scripts/eval/run.sh [options]

Options:
  --api-url URL     Assistant API URL (default: http://localhost:8080)
  --cases PATH      JSONL cases file (default: scripts/eval/cases.example.jsonl)
  --k N             Top-K for retrieval (default: 5)
  --out PATH        Output report JSON (default: ./tmp/eval/report.json)
  -h, --help        Show help
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
    --api-url)
      API_URL="${2:-}"
      shift 2
      ;;
    --cases)
      CASES_FILE="${2:-}"
      shift 2
      ;;
    --k)
      TOP_K="${2:-}"
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

if ! [[ "$TOP_K" =~ ^[0-9]+$ ]] || [[ "$TOP_K" -le 0 ]]; then
  echo "--k must be a positive integer" >&2
  exit 1
fi

if [[ ! -f "$CASES_FILE" ]]; then
  echo "cases file not found: $CASES_FILE" >&2
  exit 1
fi

require_cmd curl
require_cmd jq
require_cmd awk

mkdir -p "$(dirname "$OUT")"
tmp_cases="$(mktemp)"
trap 'rm -f "$tmp_cases"' EXIT

total=0
failed=0
sum_precision=0
sum_recall=0
sum_rr=0

while IFS= read -r line || [[ -n "$line" ]]; do
  trimmed="$(printf "%s" "$line" | tr -d '[:space:]')"
  if [[ -z "$trimmed" ]] || [[ "$line" =~ ^[[:space:]]*# ]]; then
    continue
  fi

  total=$((total + 1))

  id="$(printf "%s" "$line" | jq -r '.id // empty' 2>/dev/null || true)"
  question="$(printf "%s" "$line" | jq -r '.question // empty' 2>/dev/null || true)"
  expected_filenames="$(printf "%s" "$line" | jq -c '.expected_filenames // []' 2>/dev/null || true)"

  if [[ -z "$id" || -z "$question" || -z "$expected_filenames" ]]; then
    failed=$((failed + 1))
    continue
  fi

  expected_count="$(printf "%s" "$expected_filenames" | jq 'length')"
  if [[ "$expected_count" -le 0 ]]; then
    failed=$((failed + 1))
    continue
  fi

  payload="$(jq -nc --arg q "$question" --argjson limit "$TOP_K" '{question:$q,limit:$limit}')"
  response_file="$(mktemp)"
  http_status="$(
    curl -s -o "$response_file" -w "%{http_code}" \
      -X POST "$API_URL/v1/rag/query" \
      -H "Content-Type: application/json" \
      -d "$payload" || true
  )"
  if ! [[ "$http_status" =~ ^[0-9]{3}$ ]]; then
    http_status="000"
  fi

  if [[ "$http_status" != "200" ]]; then
    failed=$((failed + 1))
    rm -f "$response_file"
    continue
  fi

  actual_filenames="$(
    jq -c '
      [.sources[]?.filename | strings]
      | reduce .[] as $f ([]; if index($f) == null then . + [$f] else . end)
    ' "$response_file"
  )"
  rm -f "$response_file"

  relevant_count="$(
    jq -n \
      --argjson expected "$expected_filenames" \
      --argjson actual "$actual_filenames" \
      '$expected | map(. as $name | select(($actual | index($name)) != null)) | length'
  )"

  reciprocal_rank="$(
    jq -n \
      --argjson expected "$expected_filenames" \
      --argjson actual "$actual_filenames" \
      '
      [range(0; ($actual | length)) as $i | select(($expected | index($actual[$i])) != null) | $i]
      | if length > 0 then (1.0 / (.[0] + 1)) else 0 end
      '
  )"

  precision="$(awk -v rel="$relevant_count" -v k="$TOP_K" 'BEGIN { printf "%.8f", rel / k }')"
  recall="$(awk -v rel="$relevant_count" -v expected="$expected_count" 'BEGIN { printf "%.8f", rel / expected }')"

  sum_precision="$(awk -v a="$sum_precision" -v b="$precision" 'BEGIN { printf "%.8f", a + b }')"
  sum_recall="$(awk -v a="$sum_recall" -v b="$recall" 'BEGIN { printf "%.8f", a + b }')"
  sum_rr="$(awk -v a="$sum_rr" -v b="$reciprocal_rank" 'BEGIN { printf "%.8f", a + b }')"

  jq -nc \
    --arg id "$id" \
    --arg question "$question" \
    --argjson expected_filenames "$expected_filenames" \
    --argjson actual_filenames "$actual_filenames" \
    --argjson relevant_count "$relevant_count" \
    --argjson reciprocal_rank "$reciprocal_rank" \
    --argjson precision_at_k "$precision" \
    --argjson recall_at_k "$recall" \
    '{
      id: $id,
      question: $question,
      expected_filenames: $expected_filenames,
      actual_filenames: $actual_filenames,
      relevant_count: $relevant_count,
      reciprocal_rank: $reciprocal_rank,
      precision_at_k: $precision_at_k,
      recall_at_k: $recall_at_k
    }' >>"$tmp_cases"
done <"$CASES_FILE"

evaluated=$((total - failed))

if [[ "$evaluated" -le 0 ]]; then
  echo "No evaluated cases. Check API availability and case format." >&2
  exit 1
fi

mean_precision="$(awk -v s="$sum_precision" -v n="$evaluated" 'BEGIN { printf "%.8f", s / n }')"
mean_recall="$(awk -v s="$sum_recall" -v n="$evaluated" 'BEGIN { printf "%.8f", s / n }')"
mean_rr="$(awk -v s="$sum_rr" -v n="$evaluated" 'BEGIN { printf "%.8f", s / n }')"

jq -n \
  --arg api_url "$API_URL" \
  --arg cases_file "$CASES_FILE" \
  --arg generated_at "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --argjson top_k "$TOP_K" \
  --argjson total_cases "$total" \
  --argjson failed_cases "$failed" \
  --argjson evaluated_cases "$evaluated" \
  --argjson precision_at_k "$mean_precision" \
  --argjson recall_at_k "$mean_recall" \
  --argjson mrr_at_k "$mean_rr" \
  --slurpfile details "$tmp_cases" \
  '{
    generated_at: $generated_at,
    api_url: $api_url,
    cases_file: $cases_file,
    top_k: $top_k,
    summary: {
      total_cases: $total_cases,
      failed_cases: $failed_cases,
      evaluated_cases: $evaluated_cases,
      precision_at_k: $precision_at_k,
      recall_at_k: $recall_at_k,
      mrr_at_k: $mrr_at_k
    },
    cases: $details
  }' >"$OUT"

echo "Eval report saved to: $OUT"
jq '.summary' "$OUT"
