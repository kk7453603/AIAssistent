#!/usr/bin/env bash
set -euo pipefail

API_URL="${API_URL:-http://localhost:8080}"
ENV_FILE="${ENV_FILE:-.env}"
ROOT_DIR="${ROOT_DIR:-./tmp}"
REPORT_DIR="${REPORT_DIR:-./tmp/eval}"
DATASETS_DEFAULT="baseline-w01 advanced-retrieval-fast advanced-retrieval-w02 advanced-retrieval-full"
DATASETS=(${DATASETS:-$DATASETS_DEFAULT})
MODES_DEFAULT="semantic hybrid hybrid+rerank"
MODES=(${MODES:-$MODES_DEFAULT})
EVAL_K="${EVAL_K:-5}"
RAG_HYBRID_CANDIDATES="${RAG_HYBRID_CANDIDATES:-50}"
RAG_FUSION_RRF_K="${RAG_FUSION_RRF_K:-75}"
RAG_RERANK_TOP_N="${RAG_RERANK_TOP_N:-12}"
WARMUP_MODEL="${WARMUP_MODEL:-llama3.2:1b}"
OLLAMA_PROXY_URL="${OLLAMA_PROXY_URL:-http://localhost:11435}"
UPLOAD_RETRIES="${UPLOAD_RETRIES:-2}"
HARD_CASES="${HARD_CASES:-0}"
PRESERVE_ENV="${PRESERVE_ENV:-0}"

COMPOSE_FILES=(-f docker-compose.yml -f docker-compose.host-gpu.yml)
SUMMARY_CSV="${REPORT_DIR}/summary_all.csv"
COMPARE_CSV="${REPORT_DIR}/summary_compare.csv"
SUMMARY_HARD_CSV="${REPORT_DIR}/summary_hard.csv"
COMPARE_HARD_CSV="${REPORT_DIR}/summary_compare_hard.csv"

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Required command is missing: $1" >&2
    exit 1
  fi
}

mode_key() {
  local mode="$1"
  mode="${mode//+/_}"
  echo "$mode"
}

has_mode() {
  local target="$1"
  for mode in "${MODES[@]}"; do
    if [[ "$mode" == "$target" ]]; then
      return 0
    fi
  done
  return 1
}

set_env_var() {
  local key="$1"
  local value="$2"
  if [[ ! -f "$ENV_FILE" ]]; then
    printf "%s=%s\n" "$key" "$value" > "$ENV_FILE"
    return 0
  fi
  if grep -q "^${key}=" "$ENV_FILE"; then
    sed -i "s|^${key}=.*|${key}=${value}|" "$ENV_FILE"
  else
    printf "%s=%s\n" "$key" "$value" >> "$ENV_FILE"
  fi
}

apply_rag_params() {
  set_env_var "RAG_HYBRID_CANDIDATES" "$RAG_HYBRID_CANDIDATES"
  set_env_var "RAG_FUSION_RRF_K" "$RAG_FUSION_RRF_K"
  set_env_var "RAG_RERANK_TOP_N" "$RAG_RERANK_TOP_N"
}

wait_health() {
  for i in {1..60}; do
    code=$(curl -s -o /dev/null -w "%{http_code}" "$API_URL/healthz" || true)
    if [[ "$code" == "200" ]]; then
      return 0
    fi
    sleep 1
  done
  echo "healthz timeout" >&2
  return 1
}

warmup_model() {
  curl -sS "${OLLAMA_PROXY_URL%/}/api/generate" \
    -H 'Content-Type: application/json' \
    -d "{\"model\":\"$WARMUP_MODEL\",\"prompt\":\"warmup\",\"stream\":false}" \
    >/dev/null 2>&1 || true
}

restart_api() {
  docker compose "${COMPOSE_FILES[@]}" up -d --no-deps --force-recreate api >/dev/null
  wait_health
}

reset_stack() {
  docker compose "${COMPOSE_FILES[@]}" down
  docker volume rm personalaiassistent_postgres_data personalaiassistent_qdrant_data >/dev/null 2>&1 || true
  docker compose "${COMPOSE_FILES[@]}" up -d
  wait_health
  warmup_model
}

upload_dataset() {
  local dataset_dir="$1"
  local upload_log="$2"

  echo "filename,document_id,upload_http,status,error" > "$upload_log"

  for file in "$dataset_dir"/documents/*; do
    [[ -f "$file" ]] || continue
    attempt=0
    while true; do
      attempt=$((attempt + 1))
      resp=$(curl -s --connect-timeout 5 --max-time 30 -w '\n%{http_code}' -F "file=@$file" "$API_URL/v1/documents")
      body=$(printf "%s" "$resp" | head -n -1)
      code=$(printf "%s" "$resp" | tail -n 1)
      if [[ "$code" != "202" ]]; then
        echo "$(basename "$file"),,${code},upload_failed,${body//,/;}" >> "$upload_log"
        echo "upload failed: $file (attempt $attempt)" >&2
        if [[ "$attempt" -gt "$UPLOAD_RETRIES" ]]; then
          return 1
        fi
        continue
      fi
      doc_id=$(printf "%s" "$body" | jq -r '.id')
      status="processing"
      err=""
      for i in {1..180}; do
        doc=$(curl -s --connect-timeout 5 --max-time 10 "$API_URL/v1/documents/$doc_id" || true)
        status=$(printf "%s" "$doc" | jq -r '.status // empty')
        err=$(printf "%s" "$doc" | jq -r '.error // empty')
        if [[ "$status" == "ready" || "$status" == "failed" ]]; then
          break
        fi
        sleep 1
      done
      echo "$(basename "$file"),$doc_id,${code},$status,$err" >> "$upload_log"
      if [[ "$status" == "ready" ]]; then
        break
      fi
      echo "processing failed: $file status=$status err=$err (attempt $attempt)" >&2
      if [[ "$attempt" -gt "$UPLOAD_RETRIES" ]]; then
        return 1
      fi
    done
  done
}

require_cmd curl
require_cmd jq
require_cmd docker

mkdir -p "$REPORT_DIR"
echo "dataset,mode,precision_at_k,recall_at_k,mrr_at_k,total_cases,failed_cases" > "$SUMMARY_CSV"
echo "dataset,semantic_mrr,hybrid_mrr,hybrid_rerank_mrr,hybrid_mrr_delta,hybrid_rerank_mrr_delta" > "$COMPARE_CSV"
if [[ "$HARD_CASES" == "1" ]]; then
  echo "dataset,mode,precision_at_k,recall_at_k,mrr_at_k,total_cases,failed_cases" > "$SUMMARY_HARD_CSV"
  echo "dataset,semantic_mrr,hybrid_mrr,hybrid_rerank_mrr,hybrid_mrr_delta,hybrid_rerank_mrr_delta" > "$COMPARE_HARD_CSV"
fi

if [[ "$PRESERVE_ENV" != "1" && -f "$ENV_FILE" ]]; then
  cp "$ENV_FILE" "${ENV_FILE}.eval.bak"
  trap 'mv "${ENV_FILE}.eval.bak" "$ENV_FILE"' EXIT
fi

for dataset in "${DATASETS[@]}"; do
  dataset_dir="${ROOT_DIR}/${dataset}"
  cases_file="${dataset_dir}/retrieval_cases.jsonl"
  hard_cases_file="${dataset_dir}/retrieval_cases_hard.jsonl"
  upload_log="${REPORT_DIR}/upload_${dataset}.csv"

  if [[ ! -d "$dataset_dir/documents" || ! -f "$cases_file" ]]; then
    echo "Skipping dataset (missing docs or cases): $dataset_dir" >&2
    continue
  fi
  if [[ ! -s "$cases_file" && -f "$dataset_dir/manifest.csv" ]]; then
    scripts/eval/generate_cases_from_manifest.sh --manifest "$dataset_dir/manifest.csv" --out "$cases_file"
  fi
  if [[ "$HARD_CASES" == "1" && -f "$dataset_dir/manifest.csv" ]]; then
    if [[ ! -s "$hard_cases_file" ]]; then
      scripts/eval/generate_hard_cases_from_manifest.sh --manifest "$dataset_dir/manifest.csv" --out "$hard_cases_file"
    fi
  fi

  apply_rag_params
  set_env_var "RAG_RETRIEVAL_MODE" "${MODES[0]}"
  reset_stack
  upload_dataset "$dataset_dir" "$upload_log"

  for mode in "${MODES[@]}"; do
    mode_key="$(mode_key "$mode")"
    report_file="${REPORT_DIR}/report_${dataset}_${mode_key}.json"
    set_env_var "RAG_RETRIEVAL_MODE" "$mode"
    restart_api
    EVAL_K="$EVAL_K" EVAL_CASES="$cases_file" EVAL_REPORT="$report_file" make eval
    jq -r --arg ds "$dataset" --arg md "$mode" \
      '[ $ds, $md, .summary.precision_at_k, .summary.recall_at_k, .summary.mrr_at_k, .summary.total_cases, .summary.failed_cases ] | @csv' \
      "$report_file" >> "$SUMMARY_CSV"
    if [[ "$HARD_CASES" == "1" && -s "$hard_cases_file" ]]; then
      hard_report_file="${REPORT_DIR}/report_${dataset}_${mode_key}_hard.json"
      EVAL_K="$EVAL_K" EVAL_CASES="$hard_cases_file" EVAL_REPORT="$hard_report_file" make eval
      jq -r --arg ds "$dataset" --arg md "$mode" \
        '[ $ds, $md, .summary.precision_at_k, .summary.recall_at_k, .summary.mrr_at_k, .summary.total_cases, .summary.failed_cases ] | @csv' \
        "$hard_report_file" >> "$SUMMARY_HARD_CSV"
    fi
  done

  if has_mode "semantic" && has_mode "hybrid" && has_mode "hybrid+rerank"; then
    semantic_report="${REPORT_DIR}/report_${dataset}_semantic.json"
    hybrid_report="${REPORT_DIR}/report_${dataset}_hybrid.json"
    hybrid_rerank_report="${REPORT_DIR}/report_${dataset}_hybrid_rerank.json"
    compare_file="${REPORT_DIR}/compare_${dataset}.json"
    scripts/eval/compare_modes.sh \
      --semantic "$semantic_report" \
      --hybrid "$hybrid_report" \
      --hybrid-rerank "$hybrid_rerank_report" \
      --out "$compare_file" >/dev/null
    jq -r --arg ds "$dataset" \
      '[ $ds,
         .metrics.semantic.mrr_at_k,
         .metrics.hybrid.mrr_at_k,
         .metrics.hybrid_rerank.mrr_at_k,
         .deltas_vs_semantic.hybrid.mrr_at_k,
         .deltas_vs_semantic.hybrid_rerank.mrr_at_k ] | @csv' \
      "$compare_file" >> "$COMPARE_CSV"
    if [[ "$HARD_CASES" == "1" && -s "$hard_cases_file" ]]; then
      semantic_report="${REPORT_DIR}/report_${dataset}_semantic_hard.json"
      hybrid_report="${REPORT_DIR}/report_${dataset}_hybrid_hard.json"
      hybrid_rerank_report="${REPORT_DIR}/report_${dataset}_hybrid_rerank_hard.json"
      compare_file="${REPORT_DIR}/compare_${dataset}_hard.json"
      scripts/eval/compare_modes.sh \
        --semantic "$semantic_report" \
        --hybrid "$hybrid_report" \
        --hybrid-rerank "$hybrid_rerank_report" \
        --out "$compare_file" >/dev/null
      jq -r --arg ds "$dataset" \
        '[ $ds,
           .metrics.semantic.mrr_at_k,
           .metrics.hybrid.mrr_at_k,
           .metrics.hybrid_rerank.mrr_at_k,
           .deltas_vs_semantic.hybrid.mrr_at_k,
           .deltas_vs_semantic.hybrid_rerank.mrr_at_k ] | @csv' \
        "$compare_file" >> "$COMPARE_HARD_CSV"
    fi
  fi
done

echo "Summary saved to: $SUMMARY_CSV"
echo "Mode comparison summary saved to: $COMPARE_CSV"
if [[ "$HARD_CASES" == "1" ]]; then
  echo "Hard-case summary saved to: $SUMMARY_HARD_CSV"
  echo "Hard-case mode comparison summary saved to: $COMPARE_HARD_CSV"
fi
