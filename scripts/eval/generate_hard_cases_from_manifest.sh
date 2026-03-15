#!/usr/bin/env bash
set -euo pipefail

MANIFEST=""
OUT="./tmp/eval/retrieval_cases_hard.jsonl"
SEED=42
MAX_GROUP=12
MAX_SINGLE=12

usage() {
  cat <<'EOF'
Generate harder retrieval evaluation cases from a synthetic manifest.csv.

Usage:
  scripts/eval/generate_hard_cases_from_manifest.sh --manifest ./tmp/rag-300/manifest.csv \
    [--out ./tmp/eval/retrieval_cases_hard.jsonl] [--seed 42] [--max-group 12] [--max-single 12]
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
    --seed)
      SEED="${2:-}"
      shift 2
      ;;
    --max-group)
      MAX_GROUP="${2:-}"
      shift 2
      ;;
    --max-single)
      MAX_SINGLE="${2:-}"
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

require_cmd python3
mkdir -p "$(dirname "$OUT")"

MANIFEST="$MANIFEST" OUT="$OUT" SEED="$SEED" MAX_GROUP="$MAX_GROUP" MAX_SINGLE="$MAX_SINGLE" python3 - <<'PY'
import csv
import json
import os
import random
import sys

manifest = os.environ.get("MANIFEST")
out = os.environ.get("OUT")
seed = int(os.environ.get("SEED", "42"))
max_group = int(os.environ.get("MAX_GROUP", "12"))
max_single = int(os.environ.get("MAX_SINGLE", "12"))

def humanize_region(region: str) -> str:
    parts = region.replace("_", "-").split("-")
    out = []
    for p in parts:
        if p.lower() in {"eu", "us", "ap"}:
            out.append(p.upper())
        else:
            out.append(p.capitalize())
    return " ".join(out)

random.seed(seed)

rows = []
with open(manifest, newline="") as fh:
    reader = csv.DictReader(fh)
    for row in reader:
        if not row.get("filename"):
            continue
        def to_int(val):
            try:
                return int(val)
            except Exception:
                return None
        row["revenue_usd"] = to_int(row.get("revenue_usd"))
        row["cost_usd"] = to_int(row.get("cost_usd"))
        row["sla_pct"] = to_int(row.get("sla_pct"))
        row["ticket_backlog"] = to_int(row.get("ticket_backlog"))
        rows.append(row)

cases = []
seen = set()

def add_case(question: str, expected):
    expected = [e for e in expected if e]
    if not expected:
        return
    key = (question.strip(), tuple(sorted(expected)))
    if key in seen:
        return
    seen.add(key)
    cases.append({"question": question.strip(), "expected_filenames": expected})

def group_by(keys):
    grouped = {}
    for row in rows:
        k = tuple(row.get(key, "") for key in keys)
        grouped.setdefault(k, []).append(row)
    return grouped

group_defs = [
    (("region", "product"), "product", "region"),
    (("incident", "risk"), "incident", "risk"),
    (("region", "domain"), "region", "domain"),
]

for keys, a_key, b_key in group_defs:
    groups = []
    for k, items in group_by(keys).items():
        if 2 <= len(items) <= 5:
            groups.append((k, items))
    random.shuffle(groups)
    for k, items in groups[:max_group]:
        a_val = items[0].get(a_key, "")
        b_val = items[0].get(b_key, "")
        expected = [i["filename"] for i in items]
        if keys == ("region", "product"):
            question = f"Which documents relate to product {a_val} in {humanize_region(b_val)} region?"
        elif keys == ("incident", "risk"):
            question = f"Find documents with incident {a_val} and risk {b_val}."
        else:
            question = f"List documents for {b_val} in {humanize_region(a_val)}."
        add_case(question, expected)

region_groups = group_by(("region",))
for region, items in region_groups.items():
    region = region[0]
    if not region:
        continue
    items = [i for i in items if i.get("revenue_usd") is not None]
    if items:
        top = max(items, key=lambda x: x["revenue_usd"])
        add_case(
            f"In {humanize_region(region)}, which document has the highest revenue?",
            [top["filename"]],
        )

domain_groups = group_by(("domain",))
for domain, items in domain_groups.items():
    domain = domain[0]
    if not domain:
        continue
    items = [i for i in items if i.get("sla_pct") is not None]
    if items:
        worst = min(items, key=lambda x: x["sla_pct"])
        add_case(
            f"Which document shows the lowest SLA in {domain}?",
            [worst["filename"]],
        )

rows_sample = rows[:]
random.shuffle(rows_sample)
for row in rows_sample[:max_single]:
    question = (
        f"Need the report for {row.get('company','')} / {row.get('product','')} in "
        f"{humanize_region(row.get('region',''))} ({row.get('quarter','')})."
    )
    add_case(question, [row.get("filename")])

for idx, case in enumerate(cases, start=1):
    case["id"] = f"HARD{idx:04d}"

with open(out, "w", encoding="utf-8") as fh:
    for case in cases:
        fh.write(json.dumps(case, ensure_ascii=False))
        fh.write("\n")

print(f"Generated {len(cases)} hard retrieval eval cases: {out}")
PY
