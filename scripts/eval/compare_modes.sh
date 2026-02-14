#!/usr/bin/env bash
set -euo pipefail

SEMANTIC_REPORT=""
HYBRID_REPORT=""
HYBRID_RERANK_REPORT=""
OUT="./tmp/eval/modes_compare.json"

usage() {
  cat <<'USAGE'
Compare retrieval eval reports for semantic/hybrid/hybrid+rerank modes.

Usage:
  scripts/eval/compare_modes.sh \
    --semantic ./tmp/eval/semantic.json \
    --hybrid ./tmp/eval/hybrid.json \
    --hybrid-rerank ./tmp/eval/hybrid_rerank.json \
    [--out ./tmp/eval/modes_compare.json]
USAGE
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Required command is missing: $1" >&2
    exit 1
  fi
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --semantic)
      SEMANTIC_REPORT="${2:-}"
      shift 2
      ;;
    --hybrid)
      HYBRID_REPORT="${2:-}"
      shift 2
      ;;
    --hybrid-rerank)
      HYBRID_RERANK_REPORT="${2:-}"
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

if [[ -z "$SEMANTIC_REPORT" || -z "$HYBRID_REPORT" || -z "$HYBRID_RERANK_REPORT" ]]; then
  echo "--semantic, --hybrid and --hybrid-rerank are required" >&2
  usage
  exit 1
fi

for report in "$SEMANTIC_REPORT" "$HYBRID_REPORT" "$HYBRID_RERANK_REPORT"; do
  if [[ ! -f "$report" ]]; then
    echo "report not found: $report" >&2
    exit 1
  fi
done

require_cmd jq
require_cmd awk

mkdir -p "$(dirname "$OUT")"

extract_metric() {
  local file="$1"
  local metric="$2"
  jq -r ".summary.${metric} // 0" "$file"
}

delta() {
  local from="$1"
  local to="$2"
  awk -v from="$from" -v to="$to" 'BEGIN { printf "%.8f", to - from }'
}

s_p="$(extract_metric "$SEMANTIC_REPORT" precision_at_k)"
s_r="$(extract_metric "$SEMANTIC_REPORT" recall_at_k)"
s_m="$(extract_metric "$SEMANTIC_REPORT" mrr_at_k)"

h_p="$(extract_metric "$HYBRID_REPORT" precision_at_k)"
h_r="$(extract_metric "$HYBRID_REPORT" recall_at_k)"
h_m="$(extract_metric "$HYBRID_REPORT" mrr_at_k)"

hr_p="$(extract_metric "$HYBRID_RERANK_REPORT" precision_at_k)"
hr_r="$(extract_metric "$HYBRID_RERANK_REPORT" recall_at_k)"
hr_m="$(extract_metric "$HYBRID_RERANK_REPORT" mrr_at_k)"

jq -n \
  --arg generated_at "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --arg semantic_report "$SEMANTIC_REPORT" \
  --arg hybrid_report "$HYBRID_REPORT" \
  --arg hybrid_rerank_report "$HYBRID_RERANK_REPORT" \
  --argjson semantic_precision "$s_p" \
  --argjson semantic_recall "$s_r" \
  --argjson semantic_mrr "$s_m" \
  --argjson hybrid_precision "$h_p" \
  --argjson hybrid_recall "$h_r" \
  --argjson hybrid_mrr "$h_m" \
  --argjson hybrid_rerank_precision "$hr_p" \
  --argjson hybrid_rerank_recall "$hr_r" \
  --argjson hybrid_rerank_mrr "$hr_m" \
  --argjson hybrid_precision_delta "$(delta "$s_p" "$h_p")" \
  --argjson hybrid_recall_delta "$(delta "$s_r" "$h_r")" \
  --argjson hybrid_mrr_delta "$(delta "$s_m" "$h_m")" \
  --argjson hybrid_rerank_precision_delta "$(delta "$s_p" "$hr_p")" \
  --argjson hybrid_rerank_recall_delta "$(delta "$s_r" "$hr_r")" \
  --argjson hybrid_rerank_mrr_delta "$(delta "$s_m" "$hr_m")" \
  '{
    generated_at: $generated_at,
    reports: {
      semantic: $semantic_report,
      hybrid: $hybrid_report,
      hybrid_rerank: $hybrid_rerank_report
    },
    metrics: {
      semantic: {
        precision_at_k: $semantic_precision,
        recall_at_k: $semantic_recall,
        mrr_at_k: $semantic_mrr
      },
      hybrid: {
        precision_at_k: $hybrid_precision,
        recall_at_k: $hybrid_recall,
        mrr_at_k: $hybrid_mrr
      },
      hybrid_rerank: {
        precision_at_k: $hybrid_rerank_precision,
        recall_at_k: $hybrid_rerank_recall,
        mrr_at_k: $hybrid_rerank_mrr
      }
    },
    deltas_vs_semantic: {
      hybrid: {
        precision_at_k: $hybrid_precision_delta,
        recall_at_k: $hybrid_recall_delta,
        mrr_at_k: $hybrid_mrr_delta
      },
      hybrid_rerank: {
        precision_at_k: $hybrid_rerank_precision_delta,
        recall_at_k: $hybrid_rerank_recall_delta,
        mrr_at_k: $hybrid_rerank_mrr_delta
      }
    }
  }' >"$OUT"

echo "Mode comparison report saved to: $OUT"
jq '.' "$OUT"
