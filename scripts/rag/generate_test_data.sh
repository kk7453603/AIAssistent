#!/usr/bin/env bash
set -euo pipefail

COUNT=200
OUT_DIR="./tmp/rag-fixtures"
SEED=42
UPLOAD=false
WAIT_READY=false
API_URL="${API_URL:-http://localhost:8080}"
POLL_INTERVAL=2
POLL_TIMEOUT=300

usage() {
  cat <<'EOF'
Generate synthetic RAG test documents (and optionally upload them).

Usage:
  scripts/rag/generate_test_data.sh [options]

Options:
  --count N             Number of documents to generate (default: 200)
  --out-dir PATH        Output directory (default: ./tmp/rag-fixtures)
  --seed N              PRNG seed for reproducibility (default: 42)
  --upload              Upload generated files to /v1/documents
  --wait-ready          Poll /v1/documents/{id} until ready/failed (requires --upload)
  --api-url URL         Assistant API base URL (default: http://localhost:8080)
  --poll-interval SEC   Poll interval for --wait-ready (default: 2)
  --poll-timeout SEC    Poll timeout per document (default: 300)
  -h, --help            Show this help

Examples:
  scripts/rag/generate_test_data.sh --count 1000 --out-dir ./tmp/rag-1k
  scripts/rag/generate_test_data.sh --count 300 --upload --wait-ready
EOF
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Required command is missing: $1" >&2
    exit 1
  fi
}

json_get() {
  local payload="$1"
  local filter="$2"
  echo "$payload" | jq -r "$filter // empty" 2>/dev/null || true
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --count)
      COUNT="${2:-}"
      shift 2
      ;;
    --out-dir)
      OUT_DIR="${2:-}"
      shift 2
      ;;
    --seed)
      SEED="${2:-}"
      shift 2
      ;;
    --upload)
      UPLOAD=true
      shift
      ;;
    --wait-ready)
      WAIT_READY=true
      shift
      ;;
    --api-url)
      API_URL="${2:-}"
      shift 2
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
      usage
      exit 1
      ;;
  esac
done

if ! [[ "$COUNT" =~ ^[0-9]+$ ]] || [[ "$COUNT" -le 0 ]]; then
  echo "--count must be a positive integer" >&2
  exit 1
fi

if ! [[ "$SEED" =~ ^[0-9]+$ ]]; then
  echo "--seed must be a non-negative integer" >&2
  exit 1
fi

if [[ "$WAIT_READY" == true && "$UPLOAD" == false ]]; then
  echo "--wait-ready requires --upload" >&2
  exit 1
fi

require_cmd curl
if [[ "$UPLOAD" == true ]]; then
  require_cmd jq
fi

API_URL="${API_URL%/}"
DOCS_DIR="$OUT_DIR/documents"
MANIFEST_CSV="$OUT_DIR/manifest.csv"
QUERIES_TXT="$OUT_DIR/sample_queries.txt"
GROUNDED_QUERIES_TXT="$OUT_DIR/sample_queries_grounded.txt"
EVAL_CASES_CSV="$OUT_DIR/eval_cases.csv"
UPLOAD_CSV="$OUT_DIR/upload_results.csv"

mkdir -p "$DOCS_DIR"
rm -f "$MANIFEST_CSV" "$QUERIES_TXT" "$GROUNDED_QUERIES_TXT" "$EVAL_CASES_CSV" "$UPLOAD_CSV"

RANDOM="$SEED"

companies=(
  "Northstar Analytics"
  "Vertex Retail"
  "Blue Delta Logistics"
  "Apex Bio Labs"
  "Helios Media Group"
  "Arctic Mobility"
  "Nova Security Systems"
  "Granite Finance Hub"
)

domains=(
  "finance"
  "ops"
  "support"
  "compliance"
  "sales"
  "engineering"
  "hr"
  "risk"
)

regions=(
  "us-east"
  "us-west"
  "eu-central"
  "eu-north"
  "ap-south"
  "latam"
)

products=(
  "Orion"
  "Nimbus"
  "Atlas"
  "Pulse"
  "Cobalt"
  "Sierra"
  "Zenith"
  "Quasar"
)

incidents=(
  "none"
  "vendor-delay"
  "quota-miss"
  "api-latency"
  "policy-violation"
  "cost-overrun"
  "security-alert"
  "renewal-risk"
)

risk_tags=(
  "low"
  "medium"
  "high"
  "critical"
)

pick() {
  local -n arr="$1"
  local idx=$((RANDOM % ${#arr[@]}))
  echo "${arr[$idx]}"
}

calc_metric() {
  local min="$1"
  local max="$2"
  echo $(( min + (RANDOM % (max - min + 1)) ))
}

echo "filename,company,domain,region,product,incident,risk,revenue_usd,cost_usd,sla_pct,ticket_backlog,quarter" >"$MANIFEST_CSV"

for i in $(seq 1 "$COUNT"); do
  company="$(pick companies)"
  domain="$(pick domains)"
  region="$(pick regions)"
  product="$(pick products)"
  incident="$(pick incidents)"
  risk="$(pick risk_tags)"

  year=$((2023 + (RANDOM % 3)))
  quarter=$((1 + (RANDOM % 4)))
  revenue="$(calc_metric 120000 980000)"
  cost="$(calc_metric 70000 760000)"
  sla="$(calc_metric 87 100)"
  backlog="$(calc_metric 0 420)"
  nps="$(calc_metric 12 86)"
  headcount="$(calc_metric 8 160)"
  growth="$(calc_metric -15 35)"
  margin=$((revenue - cost))
  p1="$(calc_metric 0 19)"
  p2="$(calc_metric 0 44)"

  file_base="$(printf "doc_%04d_%s_%s.txt" "$i" "$domain" "$region")"
  file_path="$DOCS_DIR/$file_base"

  cat >"$file_path" <<EOF
Document ID: RAG-DOC-$i
Company: $company
Domain: $domain
Region: $region
Product: $product
Quarter: $year-Q$quarter

Operational Summary:
- Revenue USD: $revenue
- Cost USD: $cost
- Gross Margin USD: $margin
- Growth YoY Percent: $growth
- SLA Percent: $sla
- Ticket Backlog: $backlog
- NPS: $nps
- Team Headcount: $headcount

Risk and Incident:
- Incident Type: $incident
- Risk Level: $risk
- P1 Tickets: $p1
- P2 Tickets: $p2

Narrative:
The $domain function for $company in $region focused on product $product.
Main objective for $year-Q$quarter was stable execution with measurable business impact.
The quarterly review highlighted incident=$incident and risk=$risk.
This document is synthetic test data for retrieval augmented generation benchmarks.
EOF

  echo "$file_base,$company,$domain,$region,$product,$incident,$risk,$revenue,$cost,$sla,$backlog,$year-Q$quarter" >>"$MANIFEST_CSV"
done

cat >"$QUERIES_TXT" <<'EOF'
What is the SLA for product Orion in us-east?
Which documents mention security-alert incidents?
Show me files where risk is critical and backlog is above 100.
Find records with policy-violation and summarize the margin.
List companies from eu-central with growth below 0.
EOF

awk -F',' -v txtfile="$GROUNDED_QUERIES_TXT" -v csvfile="$EVAL_CASES_CSV" '
BEGIN {
  print "id,question,expected_answer" > csvfile
}
NR == 1 { next }
NR <= 9 {
  margin = $8 - $9
  add(sprintf("Какой SLA Percent у продукта %s в регионе %s?", $5, $4), sprintf("%s", $10))
  add(sprintf("Какой Incident Type встречается у %s в %s?", $5, $4), sprintf("%s", $6))
  add(sprintf("Какой Risk Level у документа %s?", $1), sprintf("%s", $7))
  add(sprintf("Какой Gross Margin USD у документа %s за %s?", $1, $12), sprintf("%d", margin))
}
END {
  if (n == 0) {
    print "No grounded queries generated: manifest is empty." >> txtfile
  }
}
function esc(s, t) {
  t = s
  gsub(/"/, "\"\"", t)
  return t
}
function add(q, a) {
  n++
  print q >> txtfile
  printf("Q%03d,\"%s\",\"%s\"\n", n, esc(q), esc(a)) >> csvfile
}
' "$MANIFEST_CSV"

echo "Generated $COUNT documents in: $DOCS_DIR"
echo "Manifest: $MANIFEST_CSV"
echo "Sample queries: $QUERIES_TXT"
echo "Grounded queries: $GROUNDED_QUERIES_TXT"
echo "Eval cases (question->answer): $EVAL_CASES_CSV"

if [[ "$UPLOAD" == false ]]; then
  exit 0
fi

echo "filename,document_id,upload_http,status,error" >"$UPLOAD_CSV"

for file in "$DOCS_DIR"/*.txt; do
  file_name="$(basename "$file")"
  response_file="$(mktemp)"
  http_status="$(
    curl -sS -o "$response_file" -w "%{http_code}" -X POST "$API_URL/v1/documents" \
      -F "file=@$file;type=text/plain"
  )"
  response="$(cat "$response_file")"
  rm -f "$response_file"

  document_id="$(json_get "$response" '.id')"
  status="$(json_get "$response" '.status')"
  err_msg="$(json_get "$response" '.error')"

  if [[ "${http_status:0:1}" != "2" || -z "$document_id" ]]; then
    [[ -z "$err_msg" ]] && err_msg="$response"
    echo "$file_name,,$http_status,upload_failed,$err_msg" >>"$UPLOAD_CSV"
    echo "UPLOAD FAILED: $file_name -> $response" >&2
    continue
  fi

  final_status="$status"
  final_error=""

  if [[ "$WAIT_READY" == true ]]; then
    started_at="$(date +%s)"
    final_status="processing"

    while true; do
      status_resp="$(curl -sS "$API_URL/v1/documents/$document_id")"
      final_status="$(json_get "$status_resp" '.status')"
      final_error="$(json_get "$status_resp" '.error')"
      if [[ "$final_status" == "ready" || "$final_status" == "failed" ]]; then
        break
      fi

      now="$(date +%s)"
      elapsed=$((now - started_at))
      if [[ "$elapsed" -ge "$POLL_TIMEOUT" ]]; then
        final_status="timeout"
        final_error="timeout waiting for ready/failed"
        break
      fi
      sleep "$POLL_INTERVAL"
    done
  fi

  echo "$file_name,$document_id,$http_status,$final_status,$final_error" >>"$UPLOAD_CSV"
done

echo "Upload results: $UPLOAD_CSV"
